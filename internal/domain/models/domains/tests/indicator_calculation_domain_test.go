package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maxCandleCount = 1000

// calculationNow is the moment every test below is asked at, so that "up to when"
// is decided by the arguments rather than by whenever the suite happens to run.
// It sits 37 minutes into an hour, which is what makes the running bucket visible.
var calculationNow = time.Date(2026, 9, 3, 8, 37, 0, 0, time.UTC)

func momentAt(clockReading string) time.Time {
	moment, parseError := time.Parse(time.RFC3339, clockReading)
	if parseError != nil {
		panic(parseError)
	}

	return moment.UTC()
}

// storedCandlesNewestFirst builds candles in the order storage hands them back. Only
// the open time matters to grouping, so nothing else is filled in beyond what a
// merge needs to have something to say.
func storedCandlesNewestFirst(openTimes ...string) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, len(openTimes))
	for _, openTime := range openTimes {
		kCandles = append(kCandles, entities.KCandle{
			Symbol:   "BTCUSDT",
			OpenTime: momentAt(openTime),
			Close:    decimal.RequireFromString("100"),
		})
	}

	return kCandles
}

// fullBucketsNewestFirst fills whole hours with all twelve of their five-minute
// candles, newest first, so that a read reaches its limit exactly on a bucket's
// worth — which is what tells the truncation rule apart from the ordinary one.
func fullBucketsNewestFirst(hours ...string) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, len(hours)*12)
	for _, hour := range hours {
		hourStart := momentAt(hour)
		for minute := 55; minute >= 0; minute -= 5 {
			kCandles = append(kCandles, entities.KCandle{
				Symbol:   "BTCUSDT",
				OpenTime: hourStart.Add(time.Duration(minute) * time.Minute),
				Close:    decimal.RequireFromString("100"),
			})
		}
	}

	return kCandles
}

func calculationRequest(
	declaredInterval string, candleCount int, endTime time.Time,
) dto.IndicatorCalculationRequestDto {
	return dto.IndicatorCalculationRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: declaredInterval,
		CandleCount:         candleCount,
		EndTime:             endTime,
		Script:              "irrelevant",
	}
}

func calculationFor(
	t *testing.T, declaredInterval string, candleCount int,
) domains.IndicatorCalculationDomain {
	t.Helper()

	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		calculationRequest(declaredInterval, candleCount, time.Time{}), maxCandleCount, calculationNow)
	require.NoError(t, validationError)

	return calculationDomain
}

func bucketOpenTimesOf(kCandleVos []vo.KCandleVo) []time.Time {
	openTimes := make([]time.Time, 0, len(kCandleVos))
	for _, kCandleVo := range kCandleVos {
		openTimes = append(openTimes, time.Unix(kCandleVo.OpenTimeUnixSeconds, 0).UTC())
	}

	return openTimes
}

func TestNewIndicatorCalculationDomainRejectsBrokenRequests(t *testing.T) {
	testCases := []struct {
		name             string
		symbol           string
		declaredInterval string
		candleCount      int
		expectedReason   string
	}{
		{
			name: "no trading symbol", symbol: "", candleCount: 30,
			expectedReason: "必須指定交易標的",
		},
		{
			name: "zero candles", symbol: "BTCUSDT", candleCount: 0,
			expectedReason: "計算根數必須大於零",
		},
		{
			name: "negative candles", symbol: "BTCUSDT", candleCount: -5,
			expectedReason: "計算根數必須大於零",
		},
		{
			name: "more candles than a single call allows", symbol: "BTCUSDT", candleCount: 1001,
			expectedReason: "超過單次可用的最大根數",
		},
		{
			name: "an interval nobody offers", symbol: "BTCUSDT", candleCount: 30,
			declaredInterval: "7m",
			expectedReason:   "彙總刻度只能是 5m、15m、1h、4h、1d 其中之一",
		},
		{
			name: "an interval that would not divide a day", symbol: "BTCUSDT", candleCount: 30,
			declaredInterval: "1w",
			expectedReason:   "彙總刻度只能是 5m、15m、1h、4h、1d 其中之一",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := calculationRequest(testCase.declaredInterval, testCase.candleCount, time.Time{})
			requestDto.Symbol = testCase.symbol

			_, validationError := domains.NewIndicatorCalculationDomain(
				requestDto, maxCandleCount, calculationNow)

			assert.ErrorIs(t, validationError, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedReason)
		})
	}
}

func TestNewIndicatorCalculationDomainCountsAggregatedCandlesNotStoredOnes(t *testing.T) {
	// A thousand daily candles is nearly three years of market and 288,000 stored
	// candles behind them; a thousand five-minute candles is three and a half days.
	// The ceiling is about how many the script is handed, so both are equally allowed
	// and neither buys nor costs room for being coarse.
	testCases := []struct {
		declaredInterval string
		candleCount      int
	}{
		{declaredInterval: "5m", candleCount: maxCandleCount},
		{declaredInterval: "1h", candleCount: maxCandleCount},
		{declaredInterval: "1d", candleCount: maxCandleCount},
		{declaredInterval: "1d", candleCount: 1},
	}

	for _, testCase := range testCases {
		t.Run(testCase.declaredInterval, func(t *testing.T) {
			_, validationError := domains.NewIndicatorCalculationDomain(
				calculationRequest(testCase.declaredInterval, testCase.candleCount, time.Time{}),
				maxCandleCount, calculationNow)

			assert.NoError(t, validationError)
		})
	}
}

func TestNewIndicatorCalculationDomainReadsTheDeclaredInterval(t *testing.T) {
	testCases := []struct {
		name             string
		declaredInterval string
		expectedInterval vo.AggregationIntervalVo
	}{
		{
			name:             "the declared one is kept for the rest of the calculation",
			declaredInterval: "1h", expectedInterval: vo.AggregationIntervalOneHour,
		},
		{
			name:             "declaring nothing means the coarseness a stored candle already has",
			declaredInterval: "", expectedInterval: vo.AggregationIntervalFiveMinutes,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain := calculationFor(t, testCase.declaredInterval, 3)

			assert.Equal(t, testCase.expectedInterval, calculationDomain.Interval().Value())
		})
	}
}

func TestReadCutoffStopsBeforeTheBucketStillRunning(t *testing.T) {
	// The cut-off is where reading stops, and everything read is therefore from a
	// bucket that has finished. A value computed from a bucket half way through
	// would change on its own as the minutes passed.
	testCases := []struct {
		name             string
		declaredInterval string
		endTime          time.Time
		expectedCutoff   string
		expectedLastUsed string
	}{
		{
			name:             "an hour that is 37 minutes old is not read at all",
			declaredInterval: "1h", endTime: time.Time{},
			expectedCutoff: "2026-09-03T08:00:00Z", expectedLastUsed: "07:00",
		},
		{
			name:             "an end time on a bucket edge takes the bucket that ends there",
			declaredInterval: "1h", endTime: momentAt("2026-09-03T08:00:00Z"),
			expectedCutoff: "2026-09-03T08:00:00Z", expectedLastUsed: "07:00",
		},
		{
			name:             "at five minutes this is the old rule of leaving out the newest candle",
			declaredInterval: "5m", endTime: time.Time{},
			expectedCutoff: "2026-09-03T08:35:00Z", expectedLastUsed: "08:30",
		},
		{
			name:             "an end time long past is judged exactly the same way",
			declaredInterval: "1h", endTime: momentAt("2025-03-01T14:30:00Z"),
			expectedCutoff: "2025-03-01T14:00:00Z", expectedLastUsed: "13:00",
		},
		{
			name:             "a day still being lived through is not read",
			declaredInterval: "1d", endTime: time.Time{},
			expectedCutoff: "2026-09-03T00:00:00Z", expectedLastUsed: "09-02",
		},
		{
			name:             "naming no end time means now",
			declaredInterval: "15m", endTime: time.Time{},
			expectedCutoff: "2026-09-03T08:30:00Z", expectedLastUsed: "08:15",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
				calculationRequest(testCase.declaredInterval, 3, testCase.endTime),
				maxCandleCount, calculationNow)

			require.NoError(t, validationError)
			assert.Equal(t, momentAt(testCase.expectedCutoff), calculationDomain.ReadCutoff(),
				"最後採用的應該是 %s 那一格", testCase.expectedLastUsed)
		})
	}
}

func TestReadCutoffTreatsAnEndTimeThatHasNotArrivedAsNow(t *testing.T) {
	// A chart scrolled a little past its right edge asks about the future as a
	// matter of course. Refusing would break that; the market simply cannot be read
	// past the present, so the answer is the same as asking about now.
	testCases := []struct {
		name    string
		endTime time.Time
	}{
		{name: "a moment later today", endTime: calculationNow.Add(time.Hour)},
		{name: "tomorrow", endTime: calculationNow.Add(24 * time.Hour)},
		{name: "one second from now", endTime: calculationNow.Add(time.Second)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
				calculationRequest("1h", 3, testCase.endTime), maxCandleCount, calculationNow)

			require.NoError(t, validationError)
			assert.Equal(t, momentAt("2026-09-03T08:00:00Z"), calculationDomain.ReadCutoff())
		})
	}
}

func TestSourceCandleLimitCoversTheBucketsAskedForPlusOneSpare(t *testing.T) {
	// The spare bucket is what a read has to give back when it stops part way
	// through the earliest one. Without it, a truncated bucket would either be
	// merged short or leave the answer one candle light.
	testCases := []struct {
		declaredInterval string
		candleCount      int
		expectedLimit    int
	}{
		{declaredInterval: "5m", candleCount: 3, expectedLimit: 4},
		{declaredInterval: "15m", candleCount: 3, expectedLimit: 12},
		{declaredInterval: "1h", candleCount: 24, expectedLimit: 300},
		{declaredInterval: "1d", candleCount: 1, expectedLimit: 576},
	}

	for _, testCase := range testCases {
		t.Run(testCase.declaredInterval, func(t *testing.T) {
			calculationDomain := calculationFor(t, testCase.declaredInterval, testCase.candleCount)

			assert.Equal(t, testCase.expectedLimit, calculationDomain.SourceCandleLimit())
		})
	}
}

func TestSelectInputCandlesHandsTheScriptOneCandlePerFinishedBucket(t *testing.T) {
	calculationDomain := calculationFor(t, "1h", 3)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(storedCandlesNewestFirst(
		"2026-09-03T07:00:00Z", "2026-09-03T06:05:00Z", "2026-09-03T06:00:00Z",
		"2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Equal(t, []time.Time{
		momentAt("2026-09-03T05:00:00Z"), momentAt("2026-09-03T06:00:00Z"), momentAt("2026-09-03T07:00:00Z"),
	}, bucketOpenTimesOf(kCandleVos), "由早到晚，每一格一根，起始時間是那一格的起點")
}

func TestSelectInputCandlesTakesTheOnesNearestTheEndTime(t *testing.T) {
	calculationDomain := calculationFor(t, "1h", 2)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(storedCandlesNewestFirst(
		"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z",
		"2026-09-03T05:00:00Z", "2026-09-03T04:00:00Z"))

	require.NoError(t, selectionError)
	assert.Equal(t, []time.Time{momentAt("2026-09-03T06:00:00Z"), momentAt("2026-09-03T07:00:00Z")},
		bucketOpenTimesOf(kCandleVos))
}

func TestSelectInputCandlesSkipsTheStretchesWithNoMarketInThem(t *testing.T) {
	// The hour in between is missing altogether. Reading a number of candles rather
	// than a stretch of time is what makes this free: the same number simply reaches
	// further back, and nothing is invented for the gap.
	calculationDomain := calculationFor(t, "1h", 2)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(storedCandlesNewestFirst(
		"2026-09-03T07:00:00Z", "2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Equal(t, []time.Time{momentAt("2026-09-03T05:00:00Z"), momentAt("2026-09-03T07:00:00Z")},
		bucketOpenTimesOf(kCandleVos))
}

func TestSelectInputCandlesCountsABucketHoldingOneCandle(t *testing.T) {
	// A bucket is not required to be full. An hour in which the market traded once
	// is an hour in which the market traded.
	calculationDomain := calculationFor(t, "1h", 1)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(
		storedCandlesNewestFirst("2026-09-03T07:35:00Z"))

	require.NoError(t, selectionError)
	require.Len(t, kCandleVos, 1)
	assert.Equal(t, momentAt("2026-09-03T07:00:00Z").Unix(), kCandleVos[0].OpenTimeUnixSeconds,
		"起始時間是那一格的起點，不是那根 K 線自己的")
}

func TestSelectInputCandlesNeverHandsOverABucketTheReadCutInHalf(t *testing.T) {
	// A read stops at a fixed number of stored candles, which does not land on a
	// bucket edge: here it reaches 06:00 but only picks up half of it. Merging that
	// half would understate the hour's opening price and its volumes, and the value
	// computed from it would look no different from a right one.
	//
	// Nothing has to detect that. Reading one bucket more than was asked for, and
	// handing over the latest ones, together put the half-read bucket out of reach.
	calculationDomain := calculationFor(t, "1h", 2)
	require.Equal(t, 36, calculationDomain.SourceCandleLimit())

	readToTheLimit := make([]entities.KCandle, 0, 36)
	readToTheLimit = append(readToTheLimit, fullBucketsNewestFirst(
		"2026-09-03T08:00:00Z", "2026-09-03T07:00:00Z")...)
	for minute := 55; minute >= 0; minute -= 5 {
		if len(readToTheLimit) == 36 {
			break
		}
		readToTheLimit = append(readToTheLimit, entities.KCandle{
			Symbol:   "BTCUSDT",
			OpenTime: momentAt("2026-09-03T06:00:00Z").Add(time.Duration(minute) * time.Minute),
			Close:    decimal.RequireFromString("100"),
		})
	}
	require.Len(t, readToTheLimit, 36)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(readToTheLimit)

	require.NoError(t, selectionError)
	assert.Equal(t,
		[]time.Time{momentAt("2026-09-03T07:00:00Z"), momentAt("2026-09-03T08:00:00Z")},
		bucketOpenTimesOf(kCandleVos),
		"只讀到一半的 06:00 那一格不在其中")
}

func TestSelectInputCandlesKeepsEveryBucketAReadThatCameUpShortFound(t *testing.T) {
	// Three buckets holding one candle each is nowhere near the limit, so the read
	// reached the end of what is stored and the earliest bucket is whole. Asking for
	// all three of them therefore succeeds.
	calculationDomain := calculationFor(t, "1h", 3)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(storedCandlesNewestFirst(
		"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Len(t, kCandleVos, 3)
}

func TestSelectInputCandlesRefusesWhenTooFewBucketsAreThere(t *testing.T) {
	// Twelve candles of a twenty-candle average still produce a number, and it looks
	// exactly like the right one. Saying so is the only way the caller finds out.
	testCases := []struct {
		name            string
		candleCount     int
		storedOpenTimes []string
		expectedMessage string
	}{
		{
			name: "short by one", candleCount: 3,
			storedOpenTimes: []string{"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z"},
			expectedMessage: "湊得出 2 根，但要求 3 根",
		},
		{
			name: "far too few", candleCount: 30,
			storedOpenTimes: []string{"2026-09-03T07:00:00Z"},
			expectedMessage: "湊得出 1 根，但要求 30 根",
		},
		{
			name: "no candles at all", candleCount: 5, storedOpenTimes: nil,
			expectedMessage: "湊得出 0 根，但要求 5 根",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain := calculationFor(t, "1h", testCase.candleCount)

			kCandleVos, selectionError := calculationDomain.SelectInputCandles(
				storedCandlesNewestFirst(testCase.storedOpenTimes...))

			assert.ErrorIs(t, selectionError, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, selectionError.Error(), testCase.expectedMessage)
			assert.Nil(t, kCandleVos, "不回傳任何部分結果")
		})
	}
}

func TestSelectInputCandlesMergesEachBucketBeforeHandingItOver(t *testing.T) {
	// What the script sees is the bucket, not the candles in it: one hour's worth of
	// five-minute candles arrives as a single candle covering that hour.
	calculationDomain := calculationFor(t, "1h", 1)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles([]entities.KCandle{
		{
			Symbol: "BTCUSDT", OpenTime: momentAt("2026-09-03T07:05:00Z"),
			Open:  decimal.RequireFromString("120"),
			High:  decimal.RequireFromString("140"),
			Low:   decimal.RequireFromString("90"),
			Close: decimal.RequireFromString("110"),
		},
		{
			Symbol: "BTCUSDT", OpenTime: momentAt("2026-09-03T07:00:00Z"),
			Open:  decimal.RequireFromString("100"),
			High:  decimal.RequireFromString("130"),
			Low:   decimal.RequireFromString("95"),
			Close: decimal.RequireFromString("120"),
		},
	})

	require.NoError(t, selectionError)
	require.Len(t, kCandleVos, 1)
	assert.InDelta(t, 100.0, kCandleVos[0].Open, 0.0001)
	assert.InDelta(t, 140.0, kCandleVos[0].High, 0.0001)
	assert.InDelta(t, 90.0, kCandleVos[0].Low, 0.0001)
	assert.InDelta(t, 110.0, kCandleVos[0].Close, 0.0001)
}

func TestNewIndicatorCalculationDomainReadsTheDeclaredResultType(t *testing.T) {
	t.Run("keeps the declared kind for the rest of the calculation", func(t *testing.T) {
		requestDto := calculationRequest("1h", 3, time.Time{})
		requestDto.ResultType = "boolList"

		calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
			requestDto, maxCandleCount, calculationNow)

		require.NoError(t, validationError)
		assert.Equal(t, vo.IndicatorResultTypeBoolList, calculationDomain.ResultType().Value())
	})

	t.Run("declaring nothing means one number per indicator", func(t *testing.T) {
		calculationDomain := calculationFor(t, "1h", 3)

		assert.Equal(t, vo.IndicatorResultTypeFloat, calculationDomain.ResultType().Value())
	})

	t.Run("refuses a kind that is not on offer", func(t *testing.T) {
		requestDto := calculationRequest("1h", 3, time.Time{})
		requestDto.ResultType = "string"

		_, validationError := domains.NewIndicatorCalculationDomain(
			requestDto, maxCandleCount, calculationNow)

		assert.ErrorIs(t, validationError, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, validationError.Error(), "指標值種類只能是")
	})

	t.Run("a broken candle count is still reported first", func(t *testing.T) {
		requestDto := calculationRequest("1h", 0, time.Time{})
		requestDto.ResultType = "string"

		_, validationError := domains.NewIndicatorCalculationDomain(
			requestDto, maxCandleCount, calculationNow)

		assert.ErrorIs(t, validationError, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, validationError.Error(), "計算根數必須大於零")
	})
}

func TestSelectInputCandlesHandsOverExactlyWhatWasAskedForAndNeverGuessesAMinimum(t *testing.T) {
	// A strategy no longer records how many candles its algorithm needs, and nothing
	// took that job over: an algorithm that needs fifty to be worth anything is
	// handed ten if ten is what was asked for. The calculation never sees the script,
	// so it has nothing to work a minimum out from — this test pins that absence,
	// because the tempting "helpful" fix is to invent one here.
	for _, candleCount := range []int{1, 3, 10} {
		calculationDomain := calculationFor(t, "1h", candleCount)
		storedOpenTimes := make([]string, 0, 20)
		for hour := 20; hour > 0; hour-- {
			storedOpenTimes = append(storedOpenTimes,
				momentAt("2026-09-02T00:00:00Z").Add(time.Duration(hour)*time.Hour).Format(time.RFC3339))
		}

		kCandleVos, selectionError := calculationDomain.SelectInputCandles(
			storedCandlesNewestFirst(storedOpenTimes...))

		require.NoError(t, selectionError)
		assert.Len(t, kCandleVos, candleCount,
			"要幾根就給幾根——系統不替算式猜它至少需要幾根")
	}
}

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

// calculationRequest asks about a stretch long enough to hold exactly that many
// slots. The tests below are written in slots because that is what the rules they
// check are about; a round-the-clock market trades every minute of the stretch, so
// the two are the same number there — which is what makes the stretch a faithful way
// to say "this many".
func calculationRequest(
	declaredInterval string, candleCount int, endTime time.Time,
) dto.IndicatorCalculationRequestDto {
	settledEndTime := endTime
	if settledEndTime.IsZero() || settledEndTime.After(calculationNow) {
		settledEndTime = calculationNow
	}

	return dto.IndicatorCalculationRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: declaredInterval,
		StartTime:           settledEndTime.Add(-slotSpan(declaredInterval, candleCount)),
		EndTime:             endTime,
		Script:              "irrelevant",
	}
}

// slotSpan is how long that many slots of that coarseness cover. A coarseness the
// system does not recognise is measured in minutes, which is never read: the
// calculation refuses the coarseness before it ever looks at the stretch.
func slotSpan(declaredInterval string, slotCount int) time.Duration {
	interval, intervalError := domains.NewAggregationIntervalDomain(declaredInterval)
	if intervalError != nil {
		return time.Duration(slotCount) * time.Minute
	}

	return time.Duration(slotCount*interval.SourceCandleCount(1)) * domains.KCandleInterval
}

func calculationFor(
	t *testing.T, declaredInterval string, candleCount int,
) domains.IndicatorCalculationDomain {
	t.Helper()

	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		calculationRequest(declaredInterval, candleCount, time.Time{}),
		cryptoMarket(), maxCandleCount, calculationNow)
	require.NoError(t, validationError)

	return calculationDomain
}

// hourlyOpenTimesEndingBefore lists whole hours reaching back that far, newest first,
// all of them finished by the moment the tests ask at — leaving out the hours named,
// counted the same way, so that a stretch can be spanned with holes in it.
//
// Reaching back N hours and skipping none gives N buckets; skipping one gives N−1
// from the same span, which is the only way to tell "no market in that hour" apart
// from "the stretch is one hour shorter".
func hourlyOpenTimesEndingBefore(hoursBackLimit int, untradedHoursBack ...int) []string {
	skipped := make(map[int]bool, len(untradedHoursBack))
	for _, hoursBack := range untradedHoursBack {
		skipped[hoursBack] = true
	}

	openTimes := make([]string, 0, hoursBackLimit)
	for hoursBack := 1; hoursBack <= hoursBackLimit; hoursBack++ {
		if skipped[hoursBack] {
			continue
		}
		openTimes = append(openTimes, calculationNow.
			Truncate(time.Hour).
			Add(-time.Duration(hoursBack)*time.Hour).
			Format(time.RFC3339))
	}

	return openTimes
}

// calculationWithLookback builds a calculation over the given span whose strategy
// declares one look-back knob, which is the only thing that moves the floor. A
// look-back of zero declares no knob at all.
func calculationWithLookback(
	t *testing.T, requestedSpan int, lookbackCount float64,
) domains.IndicatorCalculationDomain {
	t.Helper()

	requestDto := calculationRequest("1h", requestedSpan, time.Time{})
	if lookbackCount > 0 {
		requestDto.Parameters = []dto.StrategyParameterWriteDto{
			{Name: "期數", Kind: "lookbackCount", DefaultValue: lookbackCount}}
	}

	calculationDomain, validationError := domains.NewIndicatorCalculationDomain(
		requestDto, cryptoMarket(), maxCandleCount, calculationNow)
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
			name: "a stretch of no length", symbol: "BTCUSDT", candleCount: 0,
			expectedReason: "起點必須早於終點",
		},
		{
			name: "a stretch that ends before it starts", symbol: "BTCUSDT", candleCount: -5,
			expectedReason: "起點必須早於終點",
		},
		{
			name: "more candles than a single call allows", symbol: "BTCUSDT", candleCount: 1001,
			expectedReason: "超過單次可用的最大根數",
		},
		{
			name: "an interval nobody offers", symbol: "BTCUSDT", candleCount: 30,
			declaredInterval: "7m",
			expectedReason:   "彙總刻度只能是 1m、5m、15m、1h、4h、1d 其中之一",
		},
		{
			name: "an interval that would not divide a day", symbol: "BTCUSDT", candleCount: 30,
			declaredInterval: "1w",
			expectedReason:   "彙總刻度只能是 1m、5m、15m、1h、4h、1d 其中之一",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := calculationRequest(testCase.declaredInterval, testCase.candleCount, time.Time{})
			requestDto.Symbol = testCase.symbol

			_, validationError := domains.NewIndicatorCalculationDomain(
				requestDto, cryptoMarket(), maxCandleCount, calculationNow)

			assert.ErrorIs(t, validationError, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedReason)
		})
	}
}

func TestTheCeilingAcceptsExactlyItsOwnLimit(t *testing.T) {
	// The rejected side is covered above; this is the accepted side of the same line.
	// An off-by-one here would turn away the widest stretch the system does allow,
	// and nothing else would notice — the message would read perfectly sensibly.
	calculationDomain := calculationFor(t, "1m", maxCandleCount)

	assert.Equal(t, maxCandleCount, calculationDomain.CandleCount())
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
				cryptoMarket(), maxCandleCount, calculationNow)

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
			declaredInterval: "", expectedInterval: vo.AggregationIntervalOneMinute,
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
				cryptoMarket(), maxCandleCount, calculationNow)

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
				calculationRequest("1h", 3, testCase.endTime), cryptoMarket(), maxCandleCount, calculationNow)

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
		{declaredInterval: "1m", candleCount: 3, expectedLimit: 4},
		{declaredInterval: "5m", candleCount: 3, expectedLimit: 20},
		{declaredInterval: "15m", candleCount: 3, expectedLimit: 60},
		{declaredInterval: "1h", candleCount: 24, expectedLimit: 1500},
		{declaredInterval: "1d", candleCount: 1, expectedLimit: 2880},
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
	require.Equal(t, 180, calculationDomain.SourceCandleLimit())

	readToTheLimit := make([]entities.KCandle, 0, 180)
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

func TestSelectInputCandlesAnswersOverWhateverIsThere(t *testing.T) {
	// Coming up short is not the caller's mistake. How many buckets are asked for is
	// worked out from how wide a stretch is being looked at, so a chart reaching
	// further back than storage does has asked about a stretch that is only partly
	// there — and a shorter answer is the honest one. Refusing hands back nothing and
	// leaves the reader zooming around to find a coarseness that happens to fit.
	testCases := []struct {
		name                    string
		candleCount             int
		storedOpenTimes         []string
		expectedBucketOpenTimes []time.Time
	}{
		{
			name: "exactly as many as were asked for", candleCount: 3,
			storedOpenTimes: []string{
				"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"},
			expectedBucketOpenTimes: []time.Time{
				momentAt("2026-09-03T05:00:00Z"),
				momentAt("2026-09-03T06:00:00Z"),
				momentAt("2026-09-03T07:00:00Z")},
		},
		{
			name: "fewer than were asked for, so all of them", candleCount: 30,
			storedOpenTimes: []string{
				"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"},
			expectedBucketOpenTimes: []time.Time{
				momentAt("2026-09-03T05:00:00Z"),
				momentAt("2026-09-03T06:00:00Z"),
				momentAt("2026-09-03T07:00:00Z")},
		},
		{
			name: "short by one", candleCount: 3,
			storedOpenTimes: []string{"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z"},
			expectedBucketOpenTimes: []time.Time{
				momentAt("2026-09-03T06:00:00Z"),
				momentAt("2026-09-03T07:00:00Z")},
		},
		{
			name: "a single bucket against a wide request", candleCount: 100,
			storedOpenTimes:         []string{"2026-09-03T07:00:00Z"},
			expectedBucketOpenTimes: []time.Time{momentAt("2026-09-03T07:00:00Z")},
		},
		{
			name:        "more than were asked for still hands over the latest ones",
			candleCount: 2,
			storedOpenTimes: []string{
				"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"},
			expectedBucketOpenTimes: []time.Time{
				momentAt("2026-09-03T06:00:00Z"),
				momentAt("2026-09-03T07:00:00Z")},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain := calculationFor(t, "1h", testCase.candleCount)

			kCandleVos, selectionError := calculationDomain.SelectInputCandles(
				storedCandlesNewestFirst(testCase.storedOpenTimes...))

			require.NoError(t, selectionError)
			assert.Equal(t, testCase.expectedBucketOpenTimes, bucketOpenTimesOf(kCandleVos),
				"由早到晚，而且是最靠近截止時間的那幾格")
		})
	}
}

func TestSelectInputCandlesRefusesAStretchTooThinToYieldOneValue(t *testing.T) {
	// The floor is not "as many as were asked for" — it is "enough for a single
	// value". Below it there is nothing to hand over at all, so this stays a refusal,
	// and it names both counts because the way out depends on them.
	testCases := []struct {
		name              string
		lookbackCount     float64
		storedOpenTimes   []string
		expectedAvailable int
		expectedMinimum   int
	}{
		{
			name: "one bucket short of the declared look-back", lookbackCount: 3,
			storedOpenTimes:   []string{"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z"},
			expectedAvailable: 2, expectedMinimum: 3,
		},
		{
			name: "nowhere near the declared look-back", lookbackCount: 20,
			storedOpenTimes:   []string{"2026-09-03T07:00:00Z"},
			expectedAvailable: 1, expectedMinimum: 20,
		},
		{
			name: "no candles at all, with nothing declared", lookbackCount: 0,
			storedOpenTimes:   nil,
			expectedAvailable: 0, expectedMinimum: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain := calculationWithLookback(t, 5, testCase.lookbackCount)

			kCandleVos, selectionError := calculationDomain.SelectInputCandles(
				storedCandlesNewestFirst(testCase.storedOpenTimes...))

			assert.ErrorIs(t, selectionError,
				domains.ErrIndicatorCalculationCandleCoverageTooThin)
			assert.ErrorIs(t, selectionError, domains.ErrIndicatorCalculationValidation,
				"它仍然是一種呼叫端弄錯的請求")
			availableCandleCount, minimumCandleCount, isTooThin :=
				domains.CandleCoverageShortfall(selectionError)
			require.True(t, isTooThin)
			assert.Equal(t, testCase.expectedAvailable, availableCandleCount)
			assert.Equal(t, testCase.expectedMinimum, minimumCandleCount)
			assert.Nil(t, kCandleVos, "不回傳任何部分結果")
		})
	}
}

func TestSelectInputCandlesAnswersAtExactlyTheFloor(t *testing.T) {
	// The boundary is inclusive, and it has to be: a look-back of twenty produces its
	// first value on the twentieth candle, so twenty is enough for one value. Getting
	// this off by one would refuse the very stretch that just became answerable.
	calculationDomain := calculationWithLookback(t, 100, 3)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(
		storedCandlesNewestFirst(
			"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Len(t, kCandleVos, 3)
}

func TestTheFloorIsTheHungriestDeclaredLookback(t *testing.T) {
	// The floor moves with what the strategy declares, and these cases pin it through
	// the refusal rather than by asking for the number: what matters is which stretch
	// gets turned away, and the count it names is how a caller says why.
	testCases := []struct {
		name            string
		parameters      []dto.StrategyParameterWriteDto
		availableHours  int
		expectedMinimum int
	}{
		{
			name: "several declared look-backs take the hungriest",
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "短期", Kind: "lookbackCount", DefaultValue: 5},
				{Name: "長期", Kind: "lookbackCount", DefaultValue: 60},
			},
			availableHours: 59, expectedMinimum: 60,
		},
		{
			name: "a plain number declares no reach at all, so one candle is enough",
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "倍數", Kind: "number", DefaultValue: 60},
			},
			availableHours: 0, expectedMinimum: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := calculationRequest("1h", 100, time.Time{})
			requestDto.Parameters = testCase.parameters
			calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
				requestDto, cryptoMarket(), maxCandleCount, calculationNow)
			require.NoError(t, buildError)

			_, selectionError := calculationDomain.SelectInputCandles(
				storedCandlesNewestFirst(hourlyOpenTimesEndingBefore(testCase.availableHours)...))

			require.ErrorIs(t, selectionError,
				domains.ErrIndicatorCalculationCandleCoverageTooThin)
			_, minimumCandleCount, _ := domains.CandleCoverageShortfall(selectionError)
			assert.Equal(t, testCase.expectedMinimum, minimumCandleCount)
		})
	}
}

func TestTheFloorCountsOnlyBucketsThatHoldSomething(t *testing.T) {
	// Sixty hours of history, but the market never traded in one of them. That hour
	// is not a bucket, so a look-back of sixty still has nothing to say — and the
	// refusal names 59, not 60.
	//
	// The gap has to be in the middle for this to mean anything. Sixty contiguous
	// hours minus the oldest is just a shorter stretch, and that case is already
	// covered above; only a hole inside the span tells "no market in that hour" apart
	// from it, and only it would break if empty buckets were ever filled in.
	requestDto := calculationRequest("1h", 100, time.Time{})
	requestDto.Parameters = []dto.StrategyParameterWriteDto{
		{Name: "期數", Kind: "lookbackCount", DefaultValue: 60}}
	calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
		requestDto, cryptoMarket(), maxCandleCount, calculationNow)
	require.NoError(t, buildError)

	storedOpenTimes := hourlyOpenTimesEndingBefore(60, 31)
	require.Len(t, storedOpenTimes, 59, "六十小時的跨度，中間缺一小時")

	_, selectionError := calculationDomain.SelectInputCandles(
		storedCandlesNewestFirst(storedOpenTimes...))

	availableCandleCount, minimumCandleCount, isTooThin :=
		domains.CandleCoverageShortfall(selectionError)
	require.True(t, isTooThin)
	assert.Equal(t, 59, availableCandleCount, "空的那一格不算一格")
	assert.Equal(t, 60, minimumCandleCount)
}

func TestCandleCountIsWhatAFullAnswerWouldHaveTaken(t *testing.T) {
	t.Run("the span plus what the look-back reaches back over", func(t *testing.T) {
		calculationDomain := calculationWithLookback(t, 100, 20)

		assert.Equal(t, 119, calculationDomain.CandleCount())
	})

	t.Run("no look-back costs nothing extra", func(t *testing.T) {
		calculationDomain := calculationFor(t, "1h", 100)

		assert.Equal(t, 100, calculationDomain.CandleCount())
	})
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
			requestDto, cryptoMarket(), maxCandleCount, calculationNow)

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
			requestDto, cryptoMarket(), maxCandleCount, calculationNow)

		assert.ErrorIs(t, validationError, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, validationError.Error(), "指標值種類只能是")
	})

	t.Run("a stretch of no length is still reported first", func(t *testing.T) {
		requestDto := calculationRequest("1h", 0, time.Time{})
		requestDto.ResultType = "string"

		_, validationError := domains.NewIndicatorCalculationDomain(
			requestDto, cryptoMarket(), maxCandleCount, calculationNow)

		assert.ErrorIs(t, validationError, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, validationError.Error(), "起點必須早於終點")
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

// 這條式子就是使用者不必再回答「要幾根」的原因，而它錯過一次：
// `N + L − 1` 只在真的有東西要回看時才成立，沒有回看根數時它會少拿一根。
func TestInputCandleCountIsDerivedFromTheLookbackCounts(t *testing.T) {
	testCases := []struct {
		name          string
		requestedSpan int
		parameters    []dto.StrategyParameterWriteDto
		expectedInput int
	}{
		{
			name:          "要看 12 格、最大回看 20 → 拿 31 根",
			requestedSpan: 12,
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}},
			expectedInput: 31,
		},
		{
			name:          "一個回看根數都沒有 → 就是要看的那幾格，不多不少",
			requestedSpan: 12,
			parameters:    nil,
			expectedInput: 12,
		},
		{
			name:          "只有數值也一樣——數值跟拿幾根無關",
			requestedSpan: 12,
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "倍數", Kind: "number", DefaultValue: 2}},
			expectedInput: 12,
		},
		{
			name:          "好幾個回看根數只看最大的",
			requestedSpan: 12,
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "快線", Kind: "lookbackCount", DefaultValue: 20},
				{Name: "中線", Kind: "lookbackCount", DefaultValue: 50},
				{Name: "慢線", Kind: "lookbackCount", DefaultValue: 100}},
			expectedInput: 111,
		},
		{
			name:          "只看一格也拿滿回看所需",
			requestedSpan: 1,
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 100}},
			expectedInput: 100,
		},
		{
			name:          "回看一根不多花任何一根",
			requestedSpan: 12,
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 1}},
			expectedInput: 12,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
				dto.IndicatorCalculationRequestDto{
					Symbol: "BTCUSDT",
					StartTime: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC).
						Add(-time.Duration(testCase.requestedSpan) * time.Minute),
					// One minute is the length a stored candle already covers, so one
					// bucket is one candle and the read limit reads back as the input
					// count plus the spare bucket — which is what this asserts.
					AggregationInterval: "1m",
					ResultType:          "float",
					Parameters:          testCase.parameters,
				}, cryptoMarket(), 1000, time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC))

			require.NoError(t, buildError)
			// 讀取上限是「要餵給算式的根數」加上多讀的那一格，所以反推得回來。
			assert.Equal(t, testCase.expectedInput+1, calculationDomain.SourceCandleLimit())
		})
	}
}

// 上限判斷的對象是**真的要餵進去的根數**，不是呼叫端問的那個數字：
// 一段不長的區間配上很長的回看，加起來一樣會超過。
func TestTheCeilingIsJudgedAgainstWhatWillActuallyBeFed(t *testing.T) {
	_, buildError := domains.NewIndicatorCalculationDomain(
		dto.IndicatorCalculationRequestDto{
			Symbol: "BTCUSDT",
			StartTime: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC).
				Add(-10 * 5 * time.Minute),
			AggregationInterval: "5m",
			ResultType:          "float",
			Parameters: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 100}},
		}, cryptoMarket(), 50, time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC))

	require.ErrorIs(t, buildError, domains.ErrIndicatorCalculationValidation)
	assert.Contains(t, buildError.Error(), "109")
}

// taiwanCalculationRequest asks about a stretch of Taipei-time market. It is written
// in clock readings rather than in slots, because for a market that shuts the two
// are exactly the thing that no longer agree.
func taiwanCalculationRequest(
	t *testing.T, declaredInterval string, startTime string, endTime string,
	parameters []dto.StrategyParameterWriteDto,
) dto.IndicatorCalculationRequestDto {
	t.Helper()

	return dto.IndicatorCalculationRequestDto{
		Symbol:              "2330",
		AggregationInterval: declaredInterval,
		StartTime:           mustParseTime(t, startTime),
		EndTime:             mustParseTime(t, endTime),
		Script:              "irrelevant",
		Parameters:          parameters,
	}
}

// 一段時間裡要看幾格，照市場自己的交易時段數——夜裡與週末不產生格子。
// 讀取上限反推得回計算根數：一分鐘刻度下一格就是一根，上限是計算根數加多讀的那一格。
// 2026-09-07 是週一，09-11 是週五。
func TestTheSlotsAskedForFollowTheMarketsOwnHours(t *testing.T) {
	// 一分鐘刻度讓「幾格」與「幾根」是同一個數字，斷言因此讀得出格數本身。
	//
	// **起訖兩端都算在內**，所以一段整整一小時的盤中是 61 格而不是 60——
	// 從第一分鐘到第六十一分鐘，兩端各一根。以前把交易時間除以刻度長度，
	// 那個除法把右端那一根丟掉了。
	testCases := []struct {
		name              string
		startTime         string
		endTime           string
		expectedSlotCount int
	}{
		{
			name:      "a whole session",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			expectedSlotCount: 270,
		},
		{
			name:      "a whole day holds one session, not a day of slots",
			startTime: "2026-09-06T13:30:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			expectedSlotCount: 270,
		},
		{
			name:      "wholly inside a session",
			startTime: "2026-09-07T11:00:00+08:00", endTime: "2026-09-07T12:00:00+08:00",
			expectedSlotCount: 61,
		},
		{
			name:      "across one close",
			startTime: "2026-09-07T13:00:00+08:00", endTime: "2026-09-08T10:00:00+08:00",
			expectedSlotCount: 91,
		},
		{
			name:      "across a weekend",
			startTime: "2026-09-11T12:00:00+08:00", endTime: "2026-09-14T10:00:00+08:00",
			expectedSlotCount: 151,
		},
		{
			name:      "a stretch shorter than one slot still holds one",
			startTime: "2026-09-07T10:00:00+08:00", endTime: "2026-09-07T10:00:30+08:00",
			expectedSlotCount: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
				taiwanCalculationRequest(t, "1m", testCase.startTime, testCase.endTime, nil),
				taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

			require.NoError(t, buildError)
			assert.Equal(t, testCase.expectedSlotCount+1, calculationDomain.SourceCandleLimit())
		})
	}
}

// 同樣一段二十四小時，永不收盤的市場一格都不少——這一半本來就是對的，不能被改壞。
func TestAMarketThatNeverClosesStillHoldsEverySlotOfTheStretch(t *testing.T) {
	// 五分鐘刻度：整整一天是 288 格。一分鐘刻度下同一段是 1440 格，本來就超過單次上限——
	// 那是既有規則，不是這次要驗的事。
	requestDto := dto.IndicatorCalculationRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "5m",
		StartTime:           mustParseTime(t, "2026-09-06T13:30:00+08:00"),
		EndTime:             mustParseTime(t, "2026-09-07T13:30:00+08:00"),
		Script:              "irrelevant",
	}

	calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
		requestDto, cryptoMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

	require.NoError(t, buildError)
	assert.Equal(t, (288+1)*5, calculationDomain.SourceCandleLimit())
}

// 回看要的那一段歷史仍然往更早的行情取，跨過收盤是對的：計算根數是格數加回看減一。
func TestLookBackStillReachesBackPastTheClose(t *testing.T) {
	testCases := []struct {
		name                string
		startTime           string
		endTime             string
		parameters          []dto.StrategyParameterWriteDto
		expectedCandleCount int
	}{
		{
			name:      "a whole session with a twenty-bar look-back",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}},
			expectedCandleCount: 54 + 19,
		},
		{
			// 09:00 到 10:00 在五分鐘刻度上是 13 格（兩端都算），不是 12 格。
			name:      "the first hour of a session with a twenty-bar look-back",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T10:00:00+08:00",
			parameters: []dto.StrategyParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}},
			expectedCandleCount: 13 + 19,
		},
		{
			name:      "no look-back declared costs nothing extra",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			parameters:          nil,
			expectedCandleCount: 54,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
				taiwanCalculationRequest(
					t, "5m", testCase.startTime, testCase.endTime, testCase.parameters),
				taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

			require.NoError(t, buildError)
			// 五分鐘刻度下一格是五根，讀取上限是（計算根數 + 多讀的那一格）× 五。
			assert.Equal(t, (testCase.expectedCandleCount+1)*5, calculationDomain.SourceCandleLimit())
		})
	}
}

// 要看的那一段裡市場根本沒開，換什麼刻度都一樣——這是一種認得出來的拒絕。
func TestAStretchHoldingNoMarketIsRefusedAsItsOwnKind(t *testing.T) {
	testCases := []struct {
		name      string
		startTime string
		endTime   string
	}{
		{
			name:      "wholly after the close",
			startTime: "2026-09-07T14:00:00+08:00", endTime: "2026-09-07T16:00:00+08:00",
		},
		{
			name:      "a whole Saturday",
			startTime: "2026-09-12T00:00:00+08:00", endTime: "2026-09-13T00:00:00+08:00",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewIndicatorCalculationDomain(
				taiwanCalculationRequest(t, "5m", testCase.startTime, testCase.endTime, nil),
				taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

			require.ErrorIs(t, buildError, domains.ErrObservationWindowHoldsNoTrading)
			// 與「湊不出最少可算根數」是兩種不同的拒絕：出路不一樣。
			assert.NotErrorIs(t, buildError, domains.ErrIndicatorCalculationCandleCoverageTooThin)
			assert.ErrorIs(t, buildError, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, buildError.Error(), "沒有交易")
		})
	}
}

// 休市日不預先扣除：推算格數只看「哪幾天交易、幾點到幾點」，不查假日名單。
// 少掉的那一天由「計算根數」與「實際採用根數」的落差說出來，不是在這裡先扣掉。
// 2026-09-07 是週一，09-09 是週三：三個平日、三段交易時段。
func TestHolidaysAreNotDeductedFromTheSlotsAskedFor(t *testing.T) {
	calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
		taiwanCalculationRequest(
			t, "5m", "2026-09-07T09:00:00+08:00", "2026-09-09T13:30:00+08:00", nil),
		taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

	require.NoError(t, buildError)
	// 三個交易日 × 54 格 = 162 格，即使其中一天其實整天沒有交易。
	assert.Equal(t, (162+1)*5, calculationDomain.SourceCandleLimit())
}

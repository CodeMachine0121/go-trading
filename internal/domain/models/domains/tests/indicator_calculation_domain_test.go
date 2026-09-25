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

// calculationNow is 37 minutes into an hour so the still-running bucket is visible.
var calculationNow = time.Date(2026, 9, 3, 8, 37, 0, 0, time.UTC)

func momentAt(clockReading string) time.Time {
	moment, parseError := time.Parse(time.RFC3339, clockReading)
	if parseError != nil {
		panic(parseError)
	}

	return moment.UTC()
}

// storedCandlesNewestFirst mirrors storage order; only open times matter to grouping.
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

// fullBucketsNewestFirst fills whole hours with twelve five-minute candles so a read's limit lands exactly on a bucket boundary.
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

// calculationRequest spans exactly that many slots; for a round-the-clock market slots and stretch length agree.
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

// slotSpan measures unknown intervals in minutes, which is never read because the interval is refused first.
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

// hourlyOpenTimesEndingBefore lists finished hours newest first, skipping the named ones so a span can have holes (distinguishing "no market that hour" from a shorter stretch).
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

// calculationWithLookback declares one look-back parameter (none when zero), the only thing that moves the floor.
func calculationWithLookback(
	t *testing.T, requestedSpan int, lookbackCount float64,
) domains.IndicatorCalculationDomain {
	t.Helper()

	requestDto := calculationRequest("1h", requestedSpan, time.Time{})
	if lookbackCount > 0 {
		requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{
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
	// The accepted side of the ceiling; an off-by-one would refuse the widest allowed stretch unnoticed.
	calculationDomain := calculationFor(t, "1m", maxCandleCount)

	assert.Equal(t, maxCandleCount, calculationDomain.CandleCount())
}

func TestNewIndicatorCalculationDomainCountsAggregatedCandlesNotStoredOnes(t *testing.T) {
	// The ceiling limits candles handed to the script, so coarse and fine intervals are equally allowed.
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
	// Only finished buckets are read, since a running bucket's value would change on its own.
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
	// Asking about the future (e.g. a chart scrolled past its edge) is answered as asking about now, not refused.
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
	// The spare bucket covers a read that stops partway through the earliest bucket.
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
	// Reading a count of candles simply reaches further back over a missing hour; nothing is invented.
	calculationDomain := calculationFor(t, "1h", 2)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(storedCandlesNewestFirst(
		"2026-09-03T07:00:00Z", "2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Equal(t, []time.Time{momentAt("2026-09-03T05:00:00Z"), momentAt("2026-09-03T07:00:00Z")},
		bucketOpenTimesOf(kCandleVos))
}

func TestSelectInputCandlesCountsABucketHoldingOneCandle(t *testing.T) {
	// A bucket need not be full.
	calculationDomain := calculationFor(t, "1h", 1)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(
		storedCandlesNewestFirst("2026-09-03T07:35:00Z"))

	require.NoError(t, selectionError)
	require.Len(t, kCandleVos, 1)
	assert.Equal(t, momentAt("2026-09-03T07:00:00Z").Unix(), kCandleVos[0].OpenTimeUnixSeconds,
		"起始時間是那一格的起點，不是那根 K 線自己的")
}

func TestSelectInputCandlesNeverHandsOverABucketTheReadCutInHalf(t *testing.T) {
	// A read limit stopping mid-bucket (here half of 06:00) would understate that bucket; reading one spare bucket and keeping the latest ones leaves it out.
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
	// Far below the limit, the read reached the end of storage, so the earliest bucket is whole.
	calculationDomain := calculationFor(t, "1h", 3)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(storedCandlesNewestFirst(
		"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Len(t, kCandleVos, 3)
}

func TestSelectInputCandlesAnswersOverWhateverIsThere(t *testing.T) {
	// Coming up short of storage returns a shorter answer rather than a refusal, since the count derives from the viewed stretch.
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
	// Below the floor (enough for one value) it is still refused, naming both counts.
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
	// Inclusive: a twenty look-back yields its first value on the twentieth candle.
	calculationDomain := calculationWithLookback(t, 100, 3)

	kCandleVos, selectionError := calculationDomain.SelectInputCandles(
		storedCandlesNewestFirst(
			"2026-09-03T07:00:00Z", "2026-09-03T06:00:00Z", "2026-09-03T05:00:00Z"))

	require.NoError(t, selectionError)
	assert.Len(t, kCandleVos, 3)
}

func TestTheFloorIsTheHungriestDeclaredLookback(t *testing.T) {
	// The floor follows the declared look-back and is pinned through the refusal and the count it names.
	testCases := []struct {
		name            string
		parameters      []dto.StrategyScriptParameterWriteDto
		availableHours  int
		expectedMinimum int
	}{
		{
			name: "several declared look-backs take the hungriest",
			parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "短期", Kind: "lookbackCount", DefaultValue: 5},
				{Name: "長期", Kind: "lookbackCount", DefaultValue: 60},
			},
			availableHours: 59, expectedMinimum: 60,
		},
		{
			name: "a plain number declares no reach at all, so one candle is enough",
			parameters: []dto.StrategyScriptParameterWriteDto{
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
	// One untraded hour mid-span isn't a bucket, so 60 hours give 59; only a hole inside the span would break if empty buckets were filled in.
	requestDto := calculationRequest("1h", 100, time.Time{})
	requestDto.Parameters = []dto.StrategyScriptParameterWriteDto{
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
	// The script sees one merged candle per bucket.
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
	// No minimum candle count is invented: the calculation never sees the script, so it can't derive one.
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

// `N + L − 1` 只在有回看根數時成立，沒有回看時會少拿一根。
func TestInputCandleCountIsDerivedFromTheLookbackCounts(t *testing.T) {
	testCases := []struct {
		name          string
		requestedSpan int
		parameters    []dto.StrategyScriptParameterWriteDto
		expectedInput int
	}{
		{
			name:          "要看 12 格、最大回看 20 → 拿 31 根",
			requestedSpan: 12,
			parameters: []dto.StrategyScriptParameterWriteDto{
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
			parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "倍數", Kind: "number", DefaultValue: 2}},
			expectedInput: 12,
		},
		{
			name:          "好幾個回看根數只看最大的",
			requestedSpan: 12,
			parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "快線", Kind: "lookbackCount", DefaultValue: 20},
				{Name: "中線", Kind: "lookbackCount", DefaultValue: 50},
				{Name: "慢線", Kind: "lookbackCount", DefaultValue: 100}},
			expectedInput: 111,
		},
		{
			name:          "只看一格也拿滿回看所需",
			requestedSpan: 1,
			parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 100}},
			expectedInput: 100,
		},
		{
			name:          "回看一根不多花任何一根",
			requestedSpan: 12,
			parameters: []dto.StrategyScriptParameterWriteDto{
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
					// At one minute a bucket is one candle, so the read limit is the input count plus the spare bucket.
					AggregationInterval: "1m",
					ResultType:          "float",
					Parameters:          testCase.parameters,
				}, cryptoMarket(), 1000, time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC))

			require.NoError(t, buildError)
			assert.Equal(t, testCase.expectedInput+1, calculationDomain.SourceCandleLimit())
		})
	}
}

// 上限以實際要餵入的根數判斷，短區間配長回看一樣會超過。
func TestTheCeilingIsJudgedAgainstWhatWillActuallyBeFed(t *testing.T) {
	_, buildError := domains.NewIndicatorCalculationDomain(
		dto.IndicatorCalculationRequestDto{
			Symbol: "BTCUSDT",
			StartTime: time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC).
				Add(-10 * 5 * time.Minute),
			AggregationInterval: "5m",
			ResultType:          "float",
			Parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 100}},
		}, cryptoMarket(), 50, time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC))

	require.ErrorIs(t, buildError, domains.ErrIndicatorCalculationValidation)
	assert.Contains(t, buildError.Error(), "109")
}

// taiwanCalculationRequest uses Taipei clock times because slots and clock time diverge for a market that closes.
func taiwanCalculationRequest(
	t *testing.T, declaredInterval string, startTime string, endTime string,
	parameters []dto.StrategyScriptParameterWriteDto,
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

// 格數照市場自己的交易時段計算，夜裡與週末不產生格子；2026-09-07 週一、09-11 週五。
func TestTheSlotsAskedForFollowTheMarketsOwnHours(t *testing.T) {
	// 一分鐘刻度下格數即根數；起訖兩端都算，所以整整一小時的盤中是 61 格。
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

// 永不收盤的市場一格都不少。
func TestAMarketThatNeverClosesStillHoldsEverySlotOfTheStretch(t *testing.T) {
	// 五分鐘刻度下一天是 288 格（一分鐘刻度 1440 格會超過單次上限）。
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

// 回看跨過收盤往更早的行情取：計算根數是格數加回看減一。
func TestLookBackStillReachesBackPastTheClose(t *testing.T) {
	testCases := []struct {
		name                string
		startTime           string
		endTime             string
		parameters          []dto.StrategyScriptParameterWriteDto
		expectedCandleCount int
	}{
		{
			name:      "a whole session with a twenty-bar look-back",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			parameters: []dto.StrategyScriptParameterWriteDto{
				{Name: "期數", Kind: "lookbackCount", DefaultValue: 20}},
			expectedCandleCount: 54 + 19,
		},
		{
			// 09:00 到 10:00 在五分鐘刻度上是 13 格（兩端都算）。
			name:      "the first hour of a session with a twenty-bar look-back",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T10:00:00+08:00",
			parameters: []dto.StrategyScriptParameterWriteDto{
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
			// 五分鐘刻度下一格五根：讀取上限是（計算根數 + 備用一格）× 五。
			assert.Equal(t, (testCase.expectedCandleCount+1)*5, calculationDomain.SourceCandleLimit())
		})
	}
}

// 區間內市場完全沒開是一種可辨識的拒絕。
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
			// 與「湊不出最少可算根數」是不同的拒絕，出路不同。
			assert.NotErrorIs(t, buildError, domains.ErrIndicatorCalculationCandleCoverageTooThin)
			assert.ErrorIs(t, buildError, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, buildError.Error(), "沒有交易")
		})
	}
}

// 推算格數不查假日名單，只看交易時段；2026-09-07 週一至 09-09 週三共三段。
func TestHolidaysAreNotDeductedFromTheSlotsAskedFor(t *testing.T) {
	calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
		taiwanCalculationRequest(
			t, "5m", "2026-09-07T09:00:00+08:00", "2026-09-09T13:30:00+08:00", nil),
		taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

	require.NoError(t, buildError)
	// 三個交易日 × 54 格 = 162 格，即使其中一天整天沒交易。
	assert.Equal(t, (162+1)*5, calculationDomain.SourceCandleLimit())
}

// 較粗刻度下指標計算與圖表問同一段得到相同格數；一分鐘刻度恰好是舊除法也算對的情況，所以另外驗證。
func TestTheSlotsAskedForAtACoarserInterval(t *testing.T) {
	testCases := []struct {
		name              string
		declaredInterval  string
		startTime         string
		endTime           string
		expectedSlotCount int
	}{
		{
			// 四個半小時碰到世界標準時間 01、02、03、04、05 五個整點格子。
			name: "a whole session at one hour is five slots, not four", declaredInterval: "1h",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			expectedSlotCount: 5,
		},
		{
			// 同一段跨過 04:00 那條線，所以是兩格。
			name: "a whole session at four hours is two slots, not one", declaredInterval: "4h",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			expectedSlotCount: 2,
		},
		{
			name: "a whole session at one day is one slot", declaredInterval: "1d",
			startTime: "2026-09-07T09:00:00+08:00", endTime: "2026-09-07T13:30:00+08:00",
			expectedSlotCount: 1,
		},
		{
			// 除法會說不到一格，使用者要的卻是五個位置。
			name: "five sessions at one day are five slots", declaredInterval: "1d",
			startTime: "2026-09-07T00:00:00+08:00", endTime: "2026-09-12T00:00:00+08:00",
			expectedSlotCount: 5,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
				taiwanCalculationRequest(
					t, testCase.declaredInterval, testCase.startTime, testCase.endTime, nil),
				taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

			require.NoError(t, buildError)
			// 沒有回看，計算根數即格數；不看讀取上限，因為它以原始 K 線計。
			assert.Equal(t, testCase.expectedSlotCount, calculationDomain.CandleCount())
		})
	}
}

// 台股較粗刻度的長區間照格數算會超過上限，舊除法卻只算兩百多格。
func TestACoarseTaiwanWindowIsRefusedNowThatTheSlotsAreCounted(t *testing.T) {
	_, buildError := domains.NewIndicatorCalculationDomain(
		taiwanCalculationRequest(
			t, "1d", "2021-01-01T00:00:00+08:00", "2026-01-01T00:00:00+08:00", nil),
		taiwanStockMarket(), maxCandleCount, mustParseTime(t, "2026-09-14T23:00:00+08:00"))

	require.Error(t, buildError)
	assert.ErrorIs(t, buildError, domains.ErrIndicatorCalculationValidation)
}

// 全天候市場短於一格仍算有交易；格數取整為零不能當成沒有交易。
func TestAStretchShorterThanOneSlotStillHoldsTradingOnAMarketThatNeverCloses(t *testing.T) {
	calculationDomain, buildError := domains.NewIndicatorCalculationDomain(
		dto.IndicatorCalculationRequestDto{
			Symbol:              "BTCUSDT",
			AggregationInterval: "1m",
			StartTime:           calculationNow.Add(-30 * time.Second),
			EndTime:             calculationNow,
			Script:              "irrelevant",
		},
		cryptoMarket(), maxCandleCount, calculationNow)

	require.NoError(t, buildError)
	// 下限一格：不滿一格仍為一個位置取值。
	assert.Equal(t, 1, calculationDomain.CandleCount())
}

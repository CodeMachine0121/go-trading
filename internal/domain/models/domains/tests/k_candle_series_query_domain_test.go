package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const seriesQueryMaxBucketCount = 1000

func TestNewKCandleSeriesQueryDomainAcceptsARangeThatFitsTheLimit(t *testing.T) {
	testCases := []struct {
		name                      string
		startTime                 string
		endTime                   string
		interval                  string
		expectedSourceCandleLimit int
	}{
		{
			name:      "a range cut into exactly as many buckets as one query may answer with",
			startTime: "2026-09-02T00:00:00Z", endTime: "2026-09-05T11:15:00Z", interval: "5m",
			expectedSourceCandleLimit: 1000 * 5,
		},
		{
			name:      "the same start and end is one bucket",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T10:00:00Z", interval: "1h",
			expectedSourceCandleLimit: 2 * 60,
		},
		{
			name:      "a range too wide for five minutes fits comfortably at a day",
			startTime: "2026-09-02T00:00:00Z", endTime: "2026-09-05T11:20:00Z", interval: "1d",
			expectedSourceCandleLimit: 4 * 1440,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
				Symbol:    "BTCUSDT",
				StartTime: mustParseTime(t, testCase.startTime),
				EndTime:   mustParseTime(t, testCase.endTime),
				Interval:  testCase.interval,
			}, cryptoMarket(), seriesQueryMaxBucketCount)

			require.NoError(t, validationError)
			assert.Equal(t, "BTCUSDT", seriesQueryDomain.RangeQuery().Symbol())
			assert.Equal(t, mustParseTime(t, testCase.startTime), seriesQueryDomain.RangeQuery().StartTime())
			assert.Equal(t, mustParseTime(t, testCase.endTime), seriesQueryDomain.RangeQuery().EndTime())
			assert.Equal(t, testCase.expectedSourceCandleLimit, seriesQueryDomain.SourceCandleLimit())
		})
	}
}

func TestNewKCandleSeriesQueryDomainRefusesARangeCutIntoTooManyBuckets(t *testing.T) {
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-09-02T00:00:00Z"),
		EndTime:   mustParseTime(t, "2026-09-05T11:25:00Z"),
		Interval:  "5m",
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "時間區間過大，請縮小區間；若指定了彙總刻度，也可以改用更長的一種（單次最多 1000 根）")
}

func TestNewKCandleSeriesQueryDomainRefusesWhatTheRangeQueryAlreadyRefuses(t *testing.T) {
	testCases := []struct {
		name                    string
		symbol                  string
		startTime               string
		endTime                 string
		expectedMessageFragment string
	}{
		{
			name: "no trading symbol", symbol: "",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T11:00:00Z",
			expectedMessageFragment: "必須指定交易標的",
		},
		{
			name: "ending before it starts", symbol: "BTCUSDT",
			startTime: "2026-09-02T10:00:00Z", endTime: "2026-09-02T09:00:00Z",
			expectedMessageFragment: "結束時間不得早於開始時間",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
				Symbol:    testCase.symbol,
				StartTime: mustParseTime(t, testCase.startTime),
				EndTime:   mustParseTime(t, testCase.endTime),
				Interval:  "1h",
			}, cryptoMarket(), seriesQueryMaxBucketCount)

			require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
			assert.Contains(t, validationError.Error(), testCase.expectedMessageFragment)
		})
	}
}

func TestNewKCandleSeriesQueryDomainRefusesAnIntervalNobodyOffers(t *testing.T) {
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-09-02T10:00:00Z"),
		EndTime:   mustParseTime(t, "2026-09-02T11:00:00Z"),
		Interval:  "7m",
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "彙總刻度只能是")
}

func TestNewKCandleSeriesQueryDomainDeclaringNoIntervalMeansOneMinute(t *testing.T) {
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-09-02T10:00:00Z"),
		EndTime:   mustParseTime(t, "2026-09-02T10:55:00Z"),
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.NoError(t, validationError)
	assert.Equal(t, "1m", seriesQueryDomain.SeriesOf(nil).ToDto().Interval)
	// 55 one-minute buckets plus the spare bucket every read keeps so a tight limit never drops the newest bar.
	assert.Equal(t, 55+1, seriesQueryDomain.SourceCandleLimit())
}

func displayableCandleCountOf(displayableCandleCount int) *int {
	return &displayableCandleCount
}

// 台北 09:00–13:30 換算成世界標準時間是 01:00–05:30。
// 2026-09-07 是週一、09-12 是週六。
func TestSeriesQueryPicksTheFinestIntervalTheCallerCanDisplay(t *testing.T) {
	testCases := []struct {
		name                   string
		market                 domains.MarketDomain
		startTime              string
		endTime                string
		displayableCandleCount int
		expectedInterval       string
	}{
		{
			name:   "台股看一整天：那二十四小時裡只有 390 分鐘有交易",
			market: taiwanStockMarket(),
			// 前一個交易日收盤（05:30Z）往前推到今天開盤後兩小時（03:00Z）：270 + 120 分鐘。
			startTime: "2026-09-07T03:00:00Z", endTime: "2026-09-08T03:00:00Z",
			displayableCandleCount: 400, expectedInterval: "1m",
		},
		{
			name:      "加密貨幣看同樣的一整天：1440 分鐘都在成交",
			market:    cryptoMarket(),
			startTime: "2026-09-07T03:00:00Z", endTime: "2026-09-08T03:00:00Z",
			displayableCandleCount: 400, expectedInterval: "5m",
		},
		{
			name:      "台股看一個完整交易日：270 分鐘",
			market:    taiwanStockMarket(),
			startTime: "2026-09-07T01:00:00Z", endTime: "2026-09-07T05:30:00Z",
			displayableCandleCount: 400, expectedInterval: "1m",
		},
		{
			name:      "台股看五個交易日：1350 分鐘，一分鐘擺不下",
			market:    taiwanStockMarket(),
			startTime: "2026-09-07T01:00:00Z", endTime: "2026-09-11T05:30:00Z",
			displayableCandleCount: 400, expectedInterval: "5m",
		},
		{
			// 一整天的全天候行情：四小時一根還要六根，只有一天一根塞得進一格。
			name:      "邊界：只擺得下一根",
			market:    cryptoMarket(),
			startTime: "2026-09-07T00:00:00Z", endTime: "2026-09-08T00:00:00Z",
			displayableCandleCount: 1, expectedInterval: "1d",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
				dto.KCandleSeriesQueryDto{
					Symbol:                 "2330",
					StartTime:              mustParseTime(t, testCase.startTime),
					EndTime:                mustParseTime(t, testCase.endTime),
					DisplayableCandleCount: displayableCandleCountOf(testCase.displayableCandleCount),
				}, testCase.market, seriesQueryMaxBucketCount)

			require.NoError(t, validationError)
			assert.Equal(t, testCase.expectedInterval, seriesQueryDomain.SeriesOf(nil).ToDto().Interval)
		})
	}
}

func TestSeriesQueryRefusesBothWaysOfAskingAtOnce(t *testing.T) {
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:                 "BTCUSDT",
		StartTime:              mustParseTime(t, "2026-09-07T01:00:00Z"),
		EndTime:                mustParseTime(t, "2026-09-07T05:30:00Z"),
		Interval:               "5m",
		DisplayableCandleCount: displayableCandleCountOf(400),
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "只能挑一種")
}

// 零與「沒說」是兩件事：沒說時由系統挑刻度。
func TestSeriesQueryRefusesADisplayThatHoldsNothing(t *testing.T) {
	testCases := []struct {
		name                   string
		displayableCandleCount int
	}{
		{name: "零", displayableCandleCount: 0},
		{name: "負數", displayableCandleCount: -5},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
				Symbol:                 "BTCUSDT",
				StartTime:              mustParseTime(t, "2026-09-07T01:00:00Z"),
				EndTime:                mustParseTime(t, "2026-09-07T05:30:00Z"),
				DisplayableCandleCount: displayableCandleCountOf(testCase.displayableCandleCount),
			}, cryptoMarket(), seriesQueryMaxBucketCount)

			require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
			assert.Contains(t, validationError.Error(), "可顯示根數必須大於零")
		})
	}
}

// 由系統挑出來的刻度，不會反過來被系統自己的「一次要太多」擋掉。
func TestAChosenIntervalIsNeverRefusedByTheSystemsOwnCeiling(t *testing.T) {
	// 說擺得下 5000 根，而系統一次最多答 1000 根：以較嚴的那一個為準。
	// 十天的全天候行情：一分鐘 14400 根、五分鐘 2880 根、十五分鐘 960 根 ≤ 1000。
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		dto.KCandleSeriesQueryDto{
			Symbol:                 "BTCUSDT",
			StartTime:              mustParseTime(t, "2026-09-01T00:00:00Z"),
			EndTime:                mustParseTime(t, "2026-09-11T00:00:00Z"),
			DisplayableCandleCount: displayableCandleCountOf(5000),
		}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.NoError(t, validationError)
	assert.Equal(t, "15m", seriesQueryDomain.SeriesOf(nil).ToDto().Interval)
}

// 上限也以交易格子數計算，否則系統會拒絕自己挑出的刻度。
func TestTheCeilingCountsTheBucketsThatHoldTrading(t *testing.T) {
	t.Run("台股明確指定一分鐘看一整天：271 格，答得出來", func(t *testing.T) {
		seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
			dto.KCandleSeriesQueryDto{
				Symbol:    "2330",
				StartTime: mustParseTime(t, "2026-09-07T03:00:00Z"),
				EndTime:   mustParseTime(t, "2026-09-08T03:00:00Z"),
				Interval:  "1m",
			}, taiwanStockMarket(), seriesQueryMaxBucketCount)

		require.NoError(t, validationError)
		// 週一 03:00 到收盤 150 格加上週二開盤到 03:00 的 121 格共 271 格，再加一格備用。
		assert.Equal(t, 272, seriesQueryDomain.SourceCandleLimit())
	})

	t.Run("加密貨幣同一段一分鐘：1440 根，仍然要太多", func(t *testing.T) {
		_, validationError := domains.NewKCandleSeriesQueryDomain(
			dto.KCandleSeriesQueryDto{
				Symbol:    "BTCUSDT",
				StartTime: mustParseTime(t, "2026-09-07T03:00:00Z"),
				EndTime:   mustParseTime(t, "2026-09-08T03:00:00Z"),
				Interval:  "1m",
			}, cryptoMarket(), seriesQueryMaxBucketCount)

		require.ErrorIs(t, validationError, domains.ErrKCandleValidation)
		assert.Contains(t, validationError.Error(), "時間區間過大")
	})
}

// 完全沒有交易的一段不是錯誤，答案是沒有東西可畫。
func TestAStretchWithNoTradingIsNotRefused(t *testing.T) {
	testCases := []struct {
		name      string
		startTime string
		endTime   string
	}{
		{name: "整個週六", startTime: "2026-09-11T16:00:00Z", endTime: "2026-09-12T16:00:00Z"},
		{name: "整段落在收盤之後", startTime: "2026-09-07T06:00:00Z", endTime: "2026-09-07T08:00:00Z"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
				dto.KCandleSeriesQueryDto{
					Symbol:                 "2330",
					StartTime:              mustParseTime(t, testCase.startTime),
					EndTime:                mustParseTime(t, testCase.endTime),
					DisplayableCandleCount: displayableCandleCountOf(400),
				}, taiwanStockMarket(), seriesQueryMaxBucketCount)

			require.NoError(t, validationError)
			assert.Empty(t, seriesQueryDomain.SeriesOf(nil).ToDto().KCandles)
		})
	}
}

// 挑刻度不查假日名單，只看交易時段；2026-09-07 週一至 09-09 週三共三段交易時段。
func TestSeriesQueryDoesNotDeductHolidaysWhenPickingTheInterval(t *testing.T) {
	displayableCandleCount := 200

	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		dto.KCandleSeriesQueryDto{
			Symbol:                 "2330",
			StartTime:              mustParseTime(t, "2026-09-07T01:00:00Z"),
			EndTime:                mustParseTime(t, "2026-09-09T05:30:00Z"),
			DisplayableCandleCount: &displayableCandleCount,
		}, taiwanStockMarket(), seriesQueryMaxBucketCount)

	require.NoError(t, validationError)
	// 三個交易日 × 54 格 = 162 格擺得下 200 個位置，即使其中一天整天沒交易。
	assert.Equal(t, "5m", seriesQueryDomain.SeriesOf(nil).ToDto().Interval)
	assert.Equal(t, (162+1)*5, seriesQueryDomain.SourceCandleLimit())
}

func TestNewKCandleSeriesQueryDomainChoosesACoarsenessWhenTheCallerSaysNothing(t *testing.T) {
	// Callers naming neither interval nor display budget get the finest interval that fits instead of a refusal.
	testCases := []struct {
		name             string
		market           domains.MarketDomain
		startTime        string
		endTime          string
		expectedInterval string
	}{
		{
			name:   "a round-the-clock week is fifteen minutes",
			market: cryptoMarket(),
			// 10080 minutes of trading: a minute needs 10080 places and five minutes
			// 2016, both past the ceiling; fifteen needs 672.
			startTime: "2026-09-01T00:00:00Z", endTime: "2026-09-08T00:00:00Z",
			expectedInterval: "15m",
		},
		{
			name:   "the same week of a market that shuts is five minutes",
			market: taiwanStockMarket(),
			// The same seven days hold five sessions of 270 minutes: a minute needs
			// 1350 places, five minutes 270.
			startTime: "2026-08-31T00:00:00Z", endTime: "2026-09-07T00:00:00Z",
			expectedInterval: "5m",
		},
		{
			name:      "half an hour is still the finest there is",
			market:    cryptoMarket(),
			startTime: "2026-09-01T00:00:00Z", endTime: "2026-09-01T00:30:00Z",
			expectedInterval: "1m",
		},
		{
			// 1,000 trading minutes fit the ceiling by slot count though the inclusive stretch holds one more candle, so the case pins the interval rather than the count.
			name:      "a stretch holding exactly as many minutes of trading as the ceiling allows",
			market:    cryptoMarket(),
			startTime: "2026-09-01T00:00:00Z", endTime: "2026-09-01T16:40:00Z",
			expectedInterval: "1m",
		},
		{
			name:      "one minute more than the ceiling allows steps to the next coarseness",
			market:    cryptoMarket(),
			startTime: "2026-09-01T00:00:00Z", endTime: "2026-09-01T16:41:00Z",
			expectedInterval: "5m",
		},
		{
			name:      "a year is a day a candle, and is answered rather than refused",
			market:    cryptoMarket(),
			startTime: "2026-01-01T00:00:00Z", endTime: "2026-12-31T00:00:00Z",
			expectedInterval: "1d",
		},
		{
			// A no-trading stretch settles on the finest interval and answers empty rather than being refused.
			name:      "a Saturday of a market that shuts settles on the finest and answers empty",
			market:    taiwanStockMarket(),
			startTime: "2026-09-05T00:00:00Z", endTime: "2026-09-05T23:59:00Z",
			expectedInterval: "1m",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
				Symbol:    "BTCUSDT",
				StartTime: mustParseTime(t, testCase.startTime),
				EndTime:   mustParseTime(t, testCase.endTime),
			}, testCase.market, seriesQueryMaxBucketCount)

			require.NoError(t, validationError)
			assert.Equal(
				t, testCase.expectedInterval, string(seriesQueryDomain.SeriesOf(nil).ToDto().Interval))
		})
	}
}

func TestNewKCandleSeriesQueryDomainStillRefusesWhatNoCoarsenessCanHold(t *testing.T) {
	// The ceiling still refuses what no interval can fit: ten years needs 3650 daily candles.
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2016-01-01T00:00:00Z"),
		EndTime:   mustParseTime(t, "2026-01-01T00:00:00Z"),
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.Error(t, validationError)
	assert.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "時間區間過大")
}

func TestNewKCandleSeriesQueryDomainKeepsRefusingACoarsenessTheCallerNamedItself(t *testing.T) {
	// An explicitly named interval is still refused when too wide; only unspecified callers get flexibility.
	_, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:    "BTCUSDT",
		StartTime: mustParseTime(t, "2026-01-01T00:00:00Z"),
		EndTime:   mustParseTime(t, "2026-12-31T00:00:00Z"),
		Interval:  "1m",
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.Error(t, validationError)
	assert.ErrorIs(t, validationError, domains.ErrKCandleValidation)
	assert.Contains(t, validationError.Error(), "時間區間過大")
}

func TestNewKCandleSeriesQueryDomainStillHonoursADisplayBudgetWhenOneIsNamed(t *testing.T) {
	// A named display budget wins over the ceiling when tighter: 672 buckets fit 1000 but not 400, so it steps one coarser.
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(dto.KCandleSeriesQueryDto{
		Symbol:                 "BTCUSDT",
		StartTime:              mustParseTime(t, "2026-09-01T00:00:00Z"),
		EndTime:                mustParseTime(t, "2026-09-08T00:00:00Z"),
		DisplayableCandleCount: displayableCandleCountOf(400),
	}, cryptoMarket(), seriesQueryMaxBucketCount)

	require.NoError(t, validationError)
	assert.Equal(t, "1h", seriesQueryDomain.SeriesOf(nil).ToDto().Interval)
}

func TestTaiwanYearsAreRefusedNowThatTheBucketsAreCounted(t *testing.T) {
	// 台股五年以一天一根是 1304 格，舊的除法只算 244 格而放行，使「單次最多一千根」失真。
	testCases := []struct {
		name     string
		queryDto dto.KCandleSeriesQueryDto
	}{
		{
			name: "什麼都不說：挑到最粗的一天,而一天一根仍然切太多格",
			queryDto: dto.KCandleSeriesQueryDto{
				Symbol:    "2330",
				StartTime: mustParseTime(t, "2021-01-01T00:00:00Z"),
				EndTime:   mustParseTime(t, "2026-01-01T00:00:00Z"),
			},
		},
		{
			name: "自己指定一天:照原樣拒絕",
			queryDto: dto.KCandleSeriesQueryDto{
				Symbol:    "2330",
				StartTime: mustParseTime(t, "2021-01-01T00:00:00Z"),
				EndTime:   mustParseTime(t, "2026-01-01T00:00:00Z"),
				Interval:  "1d",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, validationError := domains.NewKCandleSeriesQueryDomain(
				testCase.queryDto, taiwanStockMarket(), seriesQueryMaxBucketCount)

			require.Error(t, validationError)
			assert.ErrorIs(t, validationError, domains.ErrKCandleValidation)
			assert.Contains(t, validationError.Error(), "時間區間過大")
		})
	}
}

func TestTaiwanAYearAnswersAtAFinerCoarsenessThanBefore(t *testing.T) {
	// 數格子也讓部分請求拿到更細的刻度：台股一年四小時一根是 500 格擺得下，舊除法算成 1467 格而退到一天。
	seriesQueryDomain, validationError := domains.NewKCandleSeriesQueryDomain(
		dto.KCandleSeriesQueryDto{
			Symbol:    "2330",
			StartTime: mustParseTime(t, "2026-01-01T00:00:00Z"),
			EndTime:   mustParseTime(t, "2026-12-31T00:00:00Z"),
		}, taiwanStockMarket(), seriesQueryMaxBucketCount)

	require.NoError(t, validationError)
	assert.Equal(t, "4h", seriesQueryDomain.SeriesOf(nil).ToDto().Interval)
}

package domains_test

import (
	"strconv"
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

func minuteAt(minute int) time.Time {
	return time.Date(2026, 8, 29, 9, minute, 0, 0, time.UTC)
}

// newestFirstCandles builds the candles in the order storage hands them back:
// newest first. Passing 15, 10, 5, 0 means 09:15 is the newest.
func newestFirstCandles(minutes ...int) []entities.KCandle {
	kCandles := make([]entities.KCandle, 0, len(minutes))
	for _, minute := range minutes {
		kCandles = append(kCandles, entities.KCandle{
			Symbol:   "BTCUSDT",
			OpenTime: minuteAt(minute),
			Close:    decimal.RequireFromString("100"),
		})
	}
	return kCandles
}

func calculationFor(t *testing.T, symbol string, candleCount int) domains.IndicatorCalculationDomain {
	calculationDomain, err := domains.NewIndicatorCalculationDomain(
		dto.IndicatorCalculationRequestDto{Symbol: symbol, CandleCount: candleCount, Script: "irrelevant"},
		maxCandleCount)
	require.NoError(t, err)
	return calculationDomain
}

func TestNewIndicatorCalculationDomainRejectsBrokenRequests(t *testing.T) {
	testCases := []struct {
		name           string
		symbol         string
		candleCount    int
		expectedReason string
	}{
		{name: "no trading symbol", symbol: "", candleCount: 30, expectedReason: "必須指定交易標的"},
		{name: "zero candles", symbol: "BTCUSDT", candleCount: 0, expectedReason: "計算根數必須大於零"},
		{name: "negative candles", symbol: "BTCUSDT", candleCount: -5, expectedReason: "計算根數必須大於零"},
		{
			name: "more candles than a single call allows", symbol: "BTCUSDT", candleCount: 5000,
			expectedReason: "超過單次可用的最大根數",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewIndicatorCalculationDomain(
				dto.IndicatorCalculationRequestDto{
					Symbol: testCase.symbol, CandleCount: testCase.candleCount, Script: "irrelevant",
				}, maxCandleCount)

			assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, err.Error(), testCase.expectedReason)
		})
	}
}

func TestNewIndicatorCalculationDomainAcceptsUsableRequests(t *testing.T) {
	testCases := []struct {
		name        string
		candleCount int
	}{
		{name: "a single candle", candleCount: 1},
		{name: "an ordinary count", candleCount: 30},
		{name: "exactly the maximum a single call allows", candleCount: maxCandleCount},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain, err := domains.NewIndicatorCalculationDomain(
				dto.IndicatorCalculationRequestDto{
					Symbol: "BTCUSDT", CandleCount: testCase.candleCount, Script: "irrelevant",
				}, maxCandleCount)

			assert.NoError(t, err)
			assert.Equal(t, "BTCUSDT", calculationDomain.Symbol())
			assert.Equal(t, testCase.candleCount+1, calculationDomain.CandleFetchCount())
		})
	}
}

func TestSelectInputCandlesExcludesTheNewestCandle(t *testing.T) {
	testCases := []struct {
		name                string
		candleCount         int
		storedNewestFirst   []int
		expectedOldestFirst []int
	}{
		{
			name:                "three of four, newest left out",
			candleCount:         3,
			storedNewestFirst:   []int{15, 10, 5, 0},
			expectedOldestFirst: []int{0, 5, 10},
		},
		{
			name:                "one of four, newest left out",
			candleCount:         1,
			storedNewestFirst:   []int{15, 10, 5, 0},
			expectedOldestFirst: []int{10},
		},
		{
			name:                "exactly enough once the newest is left out",
			candleCount:         2,
			storedNewestFirst:   []int{10, 5, 0},
			expectedOldestFirst: []int{0, 5},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain := calculationFor(t, "BTCUSDT", testCase.candleCount)

			kCandleVos, err := calculationDomain.SelectInputCandles(
				newestFirstCandles(testCase.storedNewestFirst...))

			assert.NoError(t, err)
			assert.Len(t, kCandleVos, len(testCase.expectedOldestFirst))
			for index, expectedMinute := range testCase.expectedOldestFirst {
				assert.Equal(t, minuteAt(expectedMinute).Unix(), kCandleVos[index].OpenTimeUnixSeconds)
			}
		})
	}
}

func TestSelectInputCandlesRefusesWhenTooFewRemain(t *testing.T) {
	testCases := []struct {
		name              string
		candleCount       int
		storedCandleCount int
		expectedUsable    int
	}{
		{name: "far too few", candleCount: 30, storedCandleCount: 10, expectedUsable: 9},
		{name: "short by exactly one", candleCount: 3, storedCandleCount: 3, expectedUsable: 2},
		{name: "only the newest candle exists", candleCount: 1, storedCandleCount: 1, expectedUsable: 0},
		{name: "no candles at all", candleCount: 5, storedCandleCount: 0, expectedUsable: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			calculationDomain := calculationFor(t, "BTCUSDT", testCase.candleCount)
			storedMinutes := make([]int, 0, testCase.storedCandleCount)
			for index := testCase.storedCandleCount - 1; index >= 0; index-- {
				storedMinutes = append(storedMinutes, index*5)
			}

			kCandleVos, err := calculationDomain.SelectInputCandles(newestFirstCandles(storedMinutes...))

			assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
			assert.Contains(t, err.Error(), "可用 "+strconv.Itoa(testCase.expectedUsable)+" 根")
			assert.Nil(t, kCandleVos)
		})
	}
}

func TestNewIndicatorCalculationDomainReadsTheDeclaredResultType(t *testing.T) {
	t.Run("keeps the declared kind for the rest of the calculation", func(t *testing.T) {
		calculationDomain, err := domains.NewIndicatorCalculationDomain(
			dto.IndicatorCalculationRequestDto{
				Symbol: "BTCUSDT", CandleCount: 3, Script: "irrelevant", ResultType: "boolList",
			},
			maxCandleCount)

		require.NoError(t, err)
		assert.Equal(t, vo.IndicatorResultTypeBoolList, calculationDomain.ResultType().Value())
	})

	t.Run("declaring nothing means one number per indicator", func(t *testing.T) {
		calculationDomain, err := domains.NewIndicatorCalculationDomain(
			dto.IndicatorCalculationRequestDto{Symbol: "BTCUSDT", CandleCount: 3, Script: "irrelevant"},
			maxCandleCount)

		require.NoError(t, err)
		assert.Equal(t, vo.IndicatorResultTypeFloat, calculationDomain.ResultType().Value())
	})

	t.Run("refuses a kind that is not on offer", func(t *testing.T) {
		_, err := domains.NewIndicatorCalculationDomain(
			dto.IndicatorCalculationRequestDto{
				Symbol: "BTCUSDT", CandleCount: 3, Script: "irrelevant", ResultType: "string",
			},
			maxCandleCount)

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "指標值種類只能是")
	})

	t.Run("a broken candle count is still reported first", func(t *testing.T) {
		_, err := domains.NewIndicatorCalculationDomain(
			dto.IndicatorCalculationRequestDto{
				Symbol: "BTCUSDT", CandleCount: 0, Script: "irrelevant", ResultType: "string",
			},
			maxCandleCount)

		assert.ErrorIs(t, err, domains.ErrIndicatorCalculationValidation)
		assert.Contains(t, err.Error(), "計算根數必須大於零")
	})
}

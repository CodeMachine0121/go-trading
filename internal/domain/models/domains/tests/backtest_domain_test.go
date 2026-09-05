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

const backtestMaxCandleCount = 1000

// backtestNow sits well after every stretch replayed below, so nothing here is refused
// merely for reaching into a bucket that has not finished.
var backtestNow = time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)

var backtestStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

func backtestRequest() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "1h",
		StartTime:           backtestStart,
		EndTime:             backtestStart.Add(5 * time.Hour),
		Script:              "the script",
		InitialCapital:      decimal.NewFromInt(10000),
		PositionSizingMode:  "allIn",
	}
}

// storedBacktestCandleAt builds a stored five-minute candle that many hours after the
// start of the stretch.
func storedBacktestCandleAt(hour int, closePrice int64) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: backtestStart.Add(time.Duration(hour) * time.Hour),
		Open:     decimal.NewFromInt(closePrice),
		High:     decimal.NewFromInt(closePrice),
		Low:      decimal.NewFromInt(closePrice),
		Close:    decimal.NewFromInt(closePrice),
	}
}

func TestNewBacktestDomain(t *testing.T) {
	testCases := []struct {
		name          string
		mutateRequest func(*dto.BacktestRequestDto)
		expectsError  bool
	}{
		{name: "a well formed request is accepted", mutateRequest: func(*dto.BacktestRequestDto) {}},
		{
			name: "a missing symbol is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.Symbol = ""
			},
			expectsError: true,
		},
		{
			name: "an unrecognised aggregation interval is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.AggregationInterval = "3h"
			},
			expectsError: true,
		},
		{
			name: "starting capital of zero is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.InitialCapital = decimal.Zero
			},
			expectsError: true,
		},
		{
			name: "negative starting capital is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.InitialCapital = decimal.NewFromInt(-1)
			},
			expectsError: true,
		},
		{
			name: "a stretch that ends before it starts is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.StartTime = backtestStart.Add(10 * time.Hour)
				requestDto.EndTime = backtestStart
			},
			expectsError: true,
		},
		{
			name: "a stretch shorter than one bucket is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.EndTime = backtestStart.Add(30 * time.Minute)
			},
			expectsError: true,
		},
		{
			name: "a stretch needing more buckets than one read allows is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.AggregationInterval = "5m"
				requestDto.EndTime = backtestStart.Add(2000 * 5 * time.Minute)
			},
			expectsError: true,
		},
		{
			name: "an unusable position sizing figure is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.PositionSizingMode = "percentage"
				requestDto.PositionSizingValue = decimal.Zero
			},
			expectsError: true,
		},
		{
			name: "a knob declared twice is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.Parameters = []dto.StrategyParameterWriteDto{
					{Name: "period", Kind: "lookbackCount", DefaultValue: 5},
					{Name: "period", Kind: "lookbackCount", DefaultValue: 9},
				}
			},
			expectsError: true,
		},
		{
			name: "a value for a knob nobody declared is refused",
			mutateRequest: func(requestDto *dto.BacktestRequestDto) {
				requestDto.ParameterValues = []dto.StrategyParameterValueDto{
					{Name: "period", Value: 9},
				}
			},
			expectsError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			requestDto := backtestRequest()
			testCase.mutateRequest(&requestDto)

			_, err := domains.NewBacktestDomain(requestDto, backtestMaxCandleCount, backtestNow)

			if testCase.expectsError {
				assert.ErrorIs(t, err, domains.ErrBacktestValidation)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestBacktestDomainReadPlan(t *testing.T) {
	t.Run("the read stops where the bucket the stretch ends in begins", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.EndTime = backtestStart.Add(5*time.Hour + 35*time.Minute)

		backtestDomain, err := domains.NewBacktestDomain(
			requestDto, backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		assert.Equal(t, backtestStart, backtestDomain.KCandleQuery().StartTime())
		assert.Equal(t, backtestStart.Add(5*time.Hour), backtestDomain.KCandleQuery().EndTime())
	})

	t.Run("an end that has not arrived is read as now", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.EndTime = backtestNow.Add(72 * time.Hour)

		backtestDomain, err := domains.NewBacktestDomain(
			requestDto, backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		assert.Equal(t, backtestNow, backtestDomain.KCandleQuery().EndTime())
	})

	t.Run("the read limit covers every candle those buckets could hold", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		// Six one-hour buckets from 00:00 to 05:00 inclusive, plus one spare, each
		// holding twelve five-minute candles.
		assert.Equal(t, 7*12, backtestDomain.SourceCandleLimit())
	})

	t.Run("a replayed script always produces one number per indicator", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		assert.Equal(t, vo.IndicatorResultTypeFloat, backtestDomain.ResultType().Value())
	})
}

func TestBacktestDomainSelectInputCandles(t *testing.T) {
	t.Run("finished buckets become the candles replayed, oldest first", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		inputKCandles, selectionError := backtestDomain.SelectInputCandles([]entities.KCandle{
			storedBacktestCandleAt(0, 100),
			storedBacktestCandleAt(1, 110),
			storedBacktestCandleAt(2, 120),
		})

		require.NoError(t, selectionError)
		require.Len(t, inputKCandles, 3)
		assert.Equal(t, backtestStart.Unix(), inputKCandles[0].OpenTimeUnixSeconds)
		assert.Equal(t, 100.0, inputKCandles[0].Close)
		assert.Equal(t, 120.0, inputKCandles[2].Close)
	})

	t.Run("the bucket the stretch ends in is left out", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.EndTime = backtestStart.Add(2 * time.Hour)

		backtestDomain, err := domains.NewBacktestDomain(
			requestDto, backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		inputKCandles, selectionError := backtestDomain.SelectInputCandles([]entities.KCandle{
			storedBacktestCandleAt(0, 100),
			storedBacktestCandleAt(1, 110),
			storedBacktestCandleAt(2, 120),
		})

		require.NoError(t, selectionError)
		assert.Len(t, inputKCandles, 2)
	})

	t.Run("exactly two candles is enough to replay", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		inputKCandles, selectionError := backtestDomain.SelectInputCandles([]entities.KCandle{
			storedBacktestCandleAt(0, 100),
			storedBacktestCandleAt(1, 110),
		})

		require.NoError(t, selectionError)
		assert.Len(t, inputKCandles, 2)
	})

	t.Run("one candle is not enough", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		_, selectionError := backtestDomain.SelectInputCandles([]entities.KCandle{
			storedBacktestCandleAt(0, 100),
		})

		assert.ErrorIs(t, selectionError, domains.ErrBacktestValidation)
	})

	t.Run("no candles at all is not enough", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		_, selectionError := backtestDomain.SelectInputCandles(nil)

		assert.ErrorIs(t, selectionError, domains.ErrBacktestValidation)
	})
}

func TestBacktestDomainSimulation(t *testing.T) {
	t.Run("the conditions travel with the walk", func(t *testing.T) {
		requestDto := backtestRequest()
		requestDto.InitialCapital = decimal.NewFromInt(20000)

		backtestDomain, err := domains.NewBacktestDomain(
			requestDto, backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		result := backtestDomain.ReplayOver(
			[]vo.KCandleVo{replayedCandleAt(0, 100), replayedCandleAt(1, 110)},
			[]map[string]vo.IndicatorValueVo{signalResultOf(1), signalResultOf(0)})

		assert.True(t, decimal.NewFromInt(20000).Equal(result.Summary.InitialCapital))
		assert.True(t, decimal.NewFromInt(22000).Equal(result.Summary.FinalEquity),
			"final equity was %s", result.Summary.FinalEquity)
	})

	t.Run("the result says which market and stretch it actually replayed", func(t *testing.T) {
		backtestDomain, err := domains.NewBacktestDomain(
			backtestRequest(), backtestMaxCandleCount, backtestNow)
		require.NoError(t, err)

		result := backtestDomain.ReplayOver(
			[]vo.KCandleVo{replayedCandleAt(0, 100), replayedCandleAt(1, 110)},
			[]map[string]vo.IndicatorValueVo{signalResultOf(0), signalResultOf(0)})

		assert.Equal(t, "BTCUSDT", result.Symbol)
		assert.Equal(t, "1h", result.Interval)
		assert.Equal(t, replayStart, result.StartTime)
		assert.Equal(t, replayStart.Add(time.Hour), result.EndTime)
	})
}

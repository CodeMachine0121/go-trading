package script_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// candleCountScript distinguishes a growing window from the same window repeated.
const candleCountScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"seen": float64(len(data))}
}
`

const lastClosePriceScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"close": data[len(data)-1].Close}
}
`

func TestExecuteForEachCandle(t *testing.T) {
	t.Run("runs once per candle, in order", func(t *testing.T) {
		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120, 130), noStrategyScriptParameters(t))

		require.NoError(t, err)
		assert.Len(t, perCandleIndicatorValues, 4)
	})

	t.Run("each run sees everything up to the candle it stands on", func(t *testing.T) {
		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120, 130), noStrategyScriptParameters(t))

		require.NoError(t, err)
		for candleIndex, indicatorValues := range perCandleIndicatorValues {
			assert.Equal(t, float64(candleIndex+1), numberOf(indicatorValues, "seen"))
		}
	})

	t.Run("the candle a run stands on is that run's last one", func(t *testing.T) {
		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), lastClosePriceScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		require.NoError(t, err)
		require.Len(t, perCandleIndicatorValues, 3)
		assert.Equal(t, 100.0, numberOf(perCandleIndicatorValues[0], "close"))
		assert.Equal(t, 110.0, numberOf(perCandleIndicatorValues[1], "close"))
		assert.Equal(t, 120.0, numberOf(perCandleIndicatorValues[2], "close"))
	})

	t.Run("a run cannot reach past the candle it stands on", func(t *testing.T) {
		// Re-slicing up to capacity would expose future candles; the furthest visible candle must be the current one.
		const reachesForTheFutureScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	everything := data[:cap(data)]
	return map[string]float64{"furthest": everything[len(everything)-1].Close}
}
`

		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), reachesForTheFutureScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120, 130), noStrategyScriptParameters(t))

		require.NoError(t, err)
		require.Len(t, perCandleIndicatorValues, 4)
		assert.Equal(t, 100.0, numberOf(perCandleIndicatorValues[0], "furthest"))
		assert.Equal(t, 110.0, numberOf(perCandleIndicatorValues[1], "furthest"))
		assert.Equal(t, 120.0, numberOf(perCandleIndicatorValues[2], "furthest"))
		assert.Equal(t, 130.0, numberOf(perCandleIndicatorValues[3], "furthest"))
	})

	t.Run("a script writing into its candles leaves the caller's candles as they were", func(t *testing.T) {
		const overwritesItsCandlesScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	for candleIndex := range data {
		data[candleIndex].Close = 999
	}
	return map[string]float64{}
}
`
		kCandles := candlesWithClosePrices(100, 110, 120)

		_, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), overwritesItsCandlesScript, resultTypeOf(t, "float"),
				kCandles, noStrategyScriptParameters(t))

		require.NoError(t, err)
		assert.Equal(t, candlesWithClosePrices(100, 110, 120), kCandles)
	})

	t.Run("no candles at all produces no results and no failure", func(t *testing.T) {
		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				nil, noStrategyScriptParameters(t))

		require.NoError(t, err)
		assert.Empty(t, perCandleIndicatorValues)
	})

	t.Run("the script is read once and then run, not read again per candle", func(t *testing.T) {
		// The counter survives between runs only if the script is read once for the whole replay.
		const countsItsOwnRunsScript = `
package main

import "indicator"

var runCount = 0

func Calculate(data []indicator.KCandle) map[string]float64 {
	runCount++
	return map[string]float64{"runs": float64(runCount)}
}
`

		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), countsItsOwnRunsScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		require.NoError(t, err)
		require.Len(t, perCandleIndicatorValues, 3)
		assert.Equal(t, 1.0, numberOf(perCandleIndicatorValues[0], "runs"))
		assert.Equal(t, 2.0, numberOf(perCandleIndicatorValues[1], "runs"))
		assert.Equal(t, 3.0, numberOf(perCandleIndicatorValues[2], "runs"))
	})

	t.Run("a script that cannot be read fails before any candle is looked at", func(t *testing.T) {
		_, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), "this is not go", resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110), noStrategyScriptParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})

	t.Run("a script with no entry point fails", func(t *testing.T) {
		_, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), "package main\n\nfunc NotCalculate() {}\n",
				resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110), noStrategyScriptParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})

	t.Run("a script failing on one candle brings the whole run down", func(t *testing.T) {
		// Fails mid-replay, since the count is zero only on the first candle.
		const failsOnFirstCandleScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	if len(data) < 2 {
		panic("not enough candles yet")
	}
	return map[string]float64{"ma": data[0].Close}
}
`

		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), failsOnFirstCandleScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Nil(t, perCandleIndicatorValues)
	})

	t.Run("a knob nobody declared is reported by name, not as a broken script", func(t *testing.T) {
		const readsAnUndeclaredKnobScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"ma": float64(indicator.LookbackCount("period"))}
}
`

		_, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), readsAnUndeclaredKnobScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110), noStrategyScriptParameters(t))

		require.ErrorIs(t, err, domains.ErrIndicatorParameterNotDeclared)
		parameterName, isUndeclared := domains.UndeclaredParameterName(err)
		assert.True(t, isUndeclared)
		assert.Equal(t, "period", parameterName)
	})

	t.Run("declared knobs are readable on every candle", func(t *testing.T) {
		const readsAKnobScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"period": float64(indicator.LookbackCount("period"))}
}
`

		parameters, buildError := domains.NewStrategyScriptParametersDomain(
			[]dto.StrategyScriptParameterWriteDto{
				{Name: "period", Kind: "lookbackCount", DefaultValue: 7},
			})
		require.NoError(t, buildError)

		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), readsAKnobScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), parameters)

		require.NoError(t, err)
		require.Len(t, perCandleIndicatorValues, 3)
		for _, indicatorValues := range perCandleIndicatorValues {
			assert.Equal(t, 7.0, numberOf(indicatorValues, "period"))
		}
	})
}

func TestExecuteForEachCandleUnderTheSignalKind(t *testing.T) {
	const signalPerCandleScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) indicator.Signal {
	if data[len(data)-1].Close > data[0].Close {
		return indicator.Buy
	}
	return indicator.Hold
}
`

	t.Run("hands back one signal per candle, in order", func(t *testing.T) {
		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), signalPerCandleScript, resultTypeOf(t, "signal"),
				candlesWithClosePrices(100, 90, 120), noStrategyScriptParameters(t))

		require.NoError(t, err)
		require.Len(t, perCandleIndicatorValues, 3)
		assert.Equal(t, vo.SignalHold, perCandleIndicatorValues[0][vo.SignalIndicatorKey].Signal)
		assert.Equal(t, vo.SignalHold, perCandleIndicatorValues[1][vo.SignalIndicatorKey].Signal)
		assert.Equal(t, vo.SignalBuy, perCandleIndicatorValues[2][vo.SignalIndicatorKey].Signal)
	})

	t.Run("a number-emitting script is refused under the signal kind", func(t *testing.T) {
		const numberScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"signal": 1}
}
`

		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), numberScript, resultTypeOf(t, "signal"),
				candlesWithClosePrices(100, 110), noStrategyScriptParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "indicator.Signal")
		assert.Nil(t, perCandleIndicatorValues)
	})

	t.Run("a signal left unset on one candle brings the whole run down", func(t *testing.T) {
		const unsetsOnceScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) indicator.Signal {
	if len(data) == 1 {
		var unset indicator.Signal
		return unset
	}
	return indicator.Hold
}
`

		perCandleIndicatorValues, err := spotIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), unsetsOnceScript, resultTypeOf(t, "signal"),
				candlesWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
		assert.Contains(t, err.Error(), "沒有設定方向")
		assert.Nil(t, perCandleIndicatorValues)
	})
}

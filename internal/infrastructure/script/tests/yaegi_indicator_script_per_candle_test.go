package script_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// candleCountScript reports how many candles it was shown, which is the one thing that
// tells a growing window apart from the same window run over and over.
const candleCountScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"seen": float64(len(data))}
}
`

// lastClosePriceScript reports the close of the candle it is standing on.
const lastClosePriceScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"close": data[len(data)-1].Close}
}
`

func TestExecuteForEachCandle(t *testing.T) {
	t.Run("runs once per candle, in order", func(t *testing.T) {
		perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120, 130), noStrategyParameters(t))

		require.NoError(t, err)
		assert.Len(t, perCandleIndicatorValues, 4)
	})

	t.Run("each run sees everything up to the candle it stands on", func(t *testing.T) {
		perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120, 130), noStrategyParameters(t))

		require.NoError(t, err)
		for candleIndex, indicatorValues := range perCandleIndicatorValues {
			assert.Equal(t, float64(candleIndex+1), numberOf(indicatorValues, "seen"))
		}
	})

	t.Run("the candle a run stands on is that run's last one", func(t *testing.T) {
		perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), lastClosePriceScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), noStrategyParameters(t))

		require.NoError(t, err)
		require.Len(t, perCandleIndicatorValues, 3)
		assert.Equal(t, 100.0, numberOf(perCandleIndicatorValues[0], "close"))
		assert.Equal(t, 110.0, numberOf(perCandleIndicatorValues[1], "close"))
		assert.Equal(t, 120.0, numberOf(perCandleIndicatorValues[2], "close"))
	})

	t.Run("no candles at all produces no results and no failure", func(t *testing.T) {
		perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), candleCountScript, resultTypeOf(t, "float"),
				nil, noStrategyParameters(t))

		require.NoError(t, err)
		assert.Empty(t, perCandleIndicatorValues)
	})

	t.Run("a script that cannot be read fails before any candle is looked at", func(t *testing.T) {
		_, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), "this is not go", resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110), noStrategyParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})

	t.Run("a script with no entry point fails", func(t *testing.T) {
		_, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), "package main\n\nfunc NotCalculate() {}\n",
				resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110), noStrategyParameters(t))

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})

	t.Run("a script failing on one candle brings the whole run down", func(t *testing.T) {
		// It divides by a count that is only zero on the very first candle, so the run
		// fails part way through rather than before it starts.
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

		perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), failsOnFirstCandleScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110, 120), noStrategyParameters(t))

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

		_, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			ExecuteForEachCandle(
				t.Context(), readsAnUndeclaredKnobScript, resultTypeOf(t, "float"),
				candlesWithClosePrices(100, 110), noStrategyParameters(t))

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

		parameters, buildError := domains.NewStrategyParametersDomain(
			[]dto.StrategyParameterWriteDto{
				{Name: "period", Kind: "lookbackCount", DefaultValue: 7},
			})
		require.NoError(t, buildError)

		perCandleIndicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
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

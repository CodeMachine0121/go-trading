package script_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func candlesWithClosePrices(closePrices ...float64) []vo.KCandleVo {
	kCandleVos := make([]vo.KCandleVo, 0, len(closePrices))
	for index, closePrice := range closePrices {
		kCandleVos = append(kCandleVos, vo.KCandleVo{
			Symbol:              "BTCUSDT",
			OpenTimeUnixSeconds: int64(1000 + index*300),
			Close:               closePrice,
		})
	}
	return kCandleVos
}

func resultTypeOf(t *testing.T, declared string) domains.IndicatorResultTypeDomain {
	t.Helper()
	resultType, err := domains.NewIndicatorResultTypeDomain(declared)
	assert.NoError(t, err)

	return resultType
}

// numberOf reads the lone number an indicator carries.
func numberOf(indicatorValues map[string]vo.IndicatorValueVo, indicatorName string) float64 {
	return indicatorValues[indicatorName].Numbers[0]
}

const averageCloseScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	sum := 0.0
	for _, candle := range data {
		sum += candle.Close
	}
	return map[string]float64{"ma": sum / float64(len(data))}
}
`

func TestExecuteProducesIndicatorValues(t *testing.T) {
	t.Run("produces a single named value", func(t *testing.T) {
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), averageCloseScript, resultTypeOf(t, "float"), candlesWithClosePrices(100, 110, 120), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Len(t, indicatorValues, 1)
		assert.Equal(t, 110.0, numberOf(indicatorValues, "ma"))
	})

	t.Run("produces several named values", func(t *testing.T) {
		highestAndLowestScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	highest := data[0].Close
	lowest := data[0].Close
	for _, candle := range data {
		if candle.Close > highest {
			highest = candle.Close
		}
		if candle.Close < lowest {
			lowest = candle.Close
		}
	}
	return map[string]float64{"high": highest, "low": lowest}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), highestAndLowestScript, resultTypeOf(t, "float"), candlesWithClosePrices(100, 110, 120), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Len(t, indicatorValues, 2)
		assert.Equal(t, 120.0, numberOf(indicatorValues, "high"))
		assert.Equal(t, 100.0, numberOf(indicatorValues, "low"))
	})

	t.Run("produces an empty set when the script names nothing", func(t *testing.T) {
		emptyScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), emptyScript, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Empty(t, indicatorValues)
	})

	t.Run("treats a script that names nothing at all as an empty set", func(t *testing.T) {
		nothingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return nil
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), nothingScript, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.NotNil(t, indicatorValues)
		assert.Empty(t, indicatorValues)
	})

	t.Run("keeps one value per name when a name is set twice", func(t *testing.T) {
		repeatedNameScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	values := map[string]float64{}
	values["ma"] = 100
	values["ma"] = 120
	return values
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), repeatedNameScript, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Len(t, indicatorValues, 1)
		assert.Equal(t, 120.0, numberOf(indicatorValues, "ma"))
	})

	t.Run("lets the script read every figure it was given", func(t *testing.T) {
		readEverythingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	candle := data[0]
	return map[string]float64{
		"close":    candle.Close,
		"high":     candle.High,
		"volume":   candle.Volume,
		"openTime": float64(candle.OpenTimeUnixSeconds),
	}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(t.Context(),
			readEverythingScript,
			resultTypeOf(t, "float"),
			[]vo.KCandleVo{{Close: 110.5, High: 120.25, Volume: 11.5, OpenTimeUnixSeconds: 1700000000}}, noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Equal(t, 110.5, numberOf(indicatorValues, "close"))
		assert.Equal(t, 120.25, numberOf(indicatorValues, "high"))
		assert.Equal(t, 11.5, numberOf(indicatorValues, "volume"))
		assert.Equal(t, 1700000000.0, numberOf(indicatorValues, "openTime"))
	})

	t.Run("lets the script sort its way to a middle value", func(t *testing.T) {
		medianScript := `
package main

import (
	"indicator"
	"sort"
)

func Calculate(data []indicator.KCandle) map[string]float64 {
	closePrices := []float64{}
	for _, candle := range data {
		closePrices = append(closePrices, candle.Close)
	}
	sort.Float64s(closePrices)
	return map[string]float64{"median": closePrices[len(closePrices)/2]}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), medianScript, resultTypeOf(t, "float"), candlesWithClosePrices(120, 100, 110), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Equal(t, 110.0, numberOf(indicatorValues, "median"))
	})

	t.Run("lets the script use common mathematics", func(t *testing.T) {
		squareRootScript := `
package main

import (
	"indicator"
	"math"
)

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"root": math.Sqrt(data[0].Close)}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), squareRootScript, resultTypeOf(t, "float"), candlesWithClosePrices(144), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Equal(t, 12.0, numberOf(indicatorValues, "root"))
	})
}

func TestExecuteRefusesScriptsThatCannotRun(t *testing.T) {
	testCases := []struct {
		name           string
		script         string
		expectedReason string
	}{
		{
			name:           "cannot be read",
			expectedReason: "算式無法解讀",
			script: `
package main

func Calculate(data []indicator.KCandle) map[string]float64 {
	this is not a calculation
`,
		},
		{
			name:           "has no entry point",
			expectedReason: "必須提供 Calculate 進入點",
			script: `
package main

import "indicator"

func SomethingElse(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{}
}
`,
		},
		{
			name:           "has an entry point of the wrong shape",
			expectedReason: "Calculate 的形式必須是",
			script: `
package main

func Calculate() string {
	return "not a set of indicator values"
}
`,
		},
		{
			name:           "fails while running",
			expectedReason: "算式執行失敗",
			script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"boom": data[999].Close}
}
`,
		},
		{
			name:           "stops itself deliberately",
			expectedReason: "算式執行失敗",
			script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	panic("this indicator refuses to be calculated")
}
`,
		},
		{
			name:           "writes into nothing",
			expectedReason: "算式執行失敗",
			script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	var values map[string]float64
	values["boom"] = 1
	return values
}
`,
		},
		{
			name:           "divides by zero",
			expectedReason: "算式執行失敗",
			script: `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	divisor := len(data) - 1
	return map[string]float64{"boom": float64(10 / divisor)}
}
`,
		},
		{
			name:           "reaches for the file system",
			expectedReason: "算式無法解讀",
			script: `
package main

import (
	"indicator"
	"os"
)

func Calculate(data []indicator.KCandle) map[string]float64 {
	os.Exit(1)
	return map[string]float64{}
}
`,
		},
		{
			name:           "reaches for the network",
			expectedReason: "算式無法解讀",
			script: `
package main

import (
	"indicator"
	"net/http"
)

func Calculate(data []indicator.KCandle) map[string]float64 {
	http.Get("http://example.com")
	return map[string]float64{}
}
`,
		},
		{
			name:           "reaches for the clock",
			expectedReason: "算式無法解讀",
			script: `
package main

import (
	"indicator"
	"time"
)

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"now": float64(time.Now().Unix())}
}
`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
				Execute(t.Context(), testCase.script, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))

			assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
			assert.Contains(t, err.Error(), testCase.expectedReason)
			assert.Nil(t, indicatorValues)
		})
	}
}

func TestExecuteGivesUpOnAScriptThatNeverFinishes(t *testing.T) {
	neverEndingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	total := 0.0
	for {
		total++
	}
	return map[string]float64{"never": total}
}
`
	allowance := 500 * time.Millisecond

	startedAt := time.Now()
	indicatorValues, err := script.NewYaegiIndicatorScriptProxy(allowance).
		Execute(t.Context(), neverEndingScript, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))
	elapsed := time.Since(startedAt)

	assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, err.Error(), "未能算完")
	assert.Nil(t, indicatorValues)
	assert.GreaterOrEqual(t, elapsed, allowance)
	assert.Less(t, elapsed, 20*allowance)
}

func TestExecuteStaysUsableAfterGivingUp(t *testing.T) {
	neverEndingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	total := 0.0
	for {
		total++
	}
	return map[string]float64{"never": total}
}
`
	indicatorScriptProxy := script.NewYaegiIndicatorScriptProxy(300 * time.Millisecond)
	_, abandonedError := indicatorScriptProxy.Execute(t.Context(), neverEndingScript, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))
	assert.ErrorIs(t, abandonedError, domains.ErrIndicatorScriptFailed)

	indicatorValues, err := indicatorScriptProxy.Execute(t.Context(), averageCloseScript, resultTypeOf(t, "float"), candlesWithClosePrices(100, 110, 120), noStrategyParameters(t))

	assert.NoError(t, err)
	assert.Equal(t, 110.0, numberOf(indicatorValues, "ma"))
}

func TestExecuteCollectsValuesInTheDeclaredKind(t *testing.T) {
	t.Run("a series of numbers keeps its order", func(t *testing.T) {
		movingAverageScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]float64 {
	closePrices := []float64{}
	for _, candle := range data {
		closePrices = append(closePrices, candle.Close)
	}
	return map[string][]float64{"line": closePrices}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), movingAverageScript, resultTypeOf(t, "floatList"), candlesWithClosePrices(100, 105, 110), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.True(t, indicatorValues["line"].IsList)
		assert.Equal(t, []float64{100, 105, 110}, indicatorValues["line"].Numbers)
		assert.Nil(t, indicatorValues["line"].Booleans)
	})

	t.Run("a lone answer is carried as an answer", func(t *testing.T) {
		crossScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]bool {
	return map[string]bool{"crossed": data[len(data)-1].Close > data[0].Close}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), crossScript, resultTypeOf(t, "bool"), candlesWithClosePrices(100, 120), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.False(t, indicatorValues["crossed"].IsList)
		assert.Equal(t, []bool{true}, indicatorValues["crossed"].Booleans)
		assert.Nil(t, indicatorValues["crossed"].Numbers)
	})

	t.Run("a negative answer is a value, not a missing one", func(t *testing.T) {
		neverCrossScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]bool {
	return map[string]bool{"crossed": false}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), neverCrossScript, resultTypeOf(t, "bool"), candlesWithClosePrices(100), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Contains(t, indicatorValues, "crossed")
		assert.Equal(t, []bool{false}, indicatorValues["crossed"].Booleans)
	})

	t.Run("a series of answers keeps its order", func(t *testing.T) {
		eachCandleRedScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]bool {
	answers := []bool{}
	for _, candle := range data {
		answers = append(answers, candle.Close > 100)
	}
	return map[string][]bool{"red": answers}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), eachCandleRedScript, resultTypeOf(t, "boolList"), candlesWithClosePrices(110, 90, 120), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.True(t, indicatorValues["red"].IsList)
		assert.Equal(t, []bool{true, false, true}, indicatorValues["red"].Booleans)
	})

	t.Run("every indicator in one calculation carries the same kind", func(t *testing.T) {
		twoSeriesScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]float64 {
	highs := []float64{}
	lows := []float64{}
	for _, candle := range data {
		highs = append(highs, candle.High)
		lows = append(lows, candle.Low)
	}
	return map[string][]float64{"highs": highs, "lows": lows}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(t.Context(),
			twoSeriesScript,
			resultTypeOf(t, "floatList"),
			[]vo.KCandleVo{{High: 120, Low: 100}, {High: 130, Low: 110}}, noStrategyParameters(t))

		assert.NoError(t, err)
		assert.True(t, indicatorValues["highs"].IsList)
		assert.True(t, indicatorValues["lows"].IsList)
		assert.Equal(t, []float64{120, 130}, indicatorValues["highs"].Numbers)
		assert.Equal(t, []float64{100, 110}, indicatorValues["lows"].Numbers)
	})

	t.Run("an empty series is a value, not a failure", func(t *testing.T) {
		emptySeriesScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]float64 {
	return map[string][]float64{"line": {}}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), emptySeriesScript, resultTypeOf(t, "floatList"), candlesWithClosePrices(100), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.Contains(t, indicatorValues, "line")
		assert.Empty(t, indicatorValues["line"].Numbers)
		assert.NotNil(t, indicatorValues["line"].Numbers)
	})

	t.Run("naming nothing gives an empty set under any kind", func(t *testing.T) {
		nothingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]bool {
	return nil
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
			Execute(t.Context(), nothingScript, resultTypeOf(t, "boolList"), candlesWithClosePrices(100), noStrategyParameters(t))

		assert.NoError(t, err)
		assert.NotNil(t, indicatorValues)
		assert.Empty(t, indicatorValues)
	})
}

func TestExecuteRefusesAScriptWhoseShapeIsNotTheDeclaredKind(t *testing.T) {
	numberScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{"ma": 110}
}
`
	seriesScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string][]float64 {
	return map[string][]float64{"line": {110}}
}
`
	answerScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]bool {
	return map[string]bool{"crossed": true}
}
`

	testCases := []struct {
		name          string
		declared      string
		script        string
		expectedShape string
	}{
		{
			name:     "one number was declared but a series came back",
			declared: "float", script: seriesScript, expectedShape: "map[string]float64",
		},
		{
			name:     "a series of answers was declared but a number came back",
			declared: "boolList", script: numberScript, expectedShape: "map[string][]bool",
		},
		{
			name:     "a series of numbers was declared but an answer came back",
			declared: "floatList", script: answerScript, expectedShape: "map[string][]float64",
		},
		{
			name:     "one answer was declared but a number came back",
			declared: "bool", script: numberScript, expectedShape: "map[string]bool",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).
				Execute(t.Context(), testCase.script, resultTypeOf(t, testCase.declared), candlesWithClosePrices(100, 110), noStrategyParameters(t))

			assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
			assert.Contains(t, err.Error(), "Calculate 的形式必須是")
			assert.Contains(t, err.Error(), testCase.expectedShape)
			assert.Contains(t, err.Error(), testCase.declared)
			assert.Nil(t, indicatorValues)
		})
	}
}

// Outliving the allowance and being abandoned by whoever asked both end the run
// through the same context, so the reason has to be carried rather than guessed at.
// Reported as the wrong one, "your formula is too slow" would be told to somebody
// whose formula was never given the chance to be slow.
func TestExecuteGivesUpWhenTheCallerGoesAway(t *testing.T) {
	neverEndingScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	total := 0.0
	for {
		total++
	}
	return map[string]float64{"never": total}
}
`
	// Long enough that reaching it would be the wrong reason to have stopped.
	generousAllowance := 30 * time.Second
	callerWentAway, abandonTheCall := context.WithCancel(t.Context())
	go func() {
		time.Sleep(100 * time.Millisecond)
		abandonTheCall()
	}()

	startedAt := time.Now()
	indicatorValues, err := script.NewYaegiIndicatorScriptProxy(generousAllowance).
		Execute(callerWentAway, neverEndingScript, resultTypeOf(t, "float"), candlesWithClosePrices(100), noStrategyParameters(t))
	elapsed := time.Since(startedAt)

	assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, err.Error(), "請求已經結束")
	assert.NotContains(t, err.Error(), "未能算完")
	assert.Nil(t, indicatorValues)
	assert.Less(t, elapsed, generousAllowance)
}

// noStrategyParameters is an algorithm with no knobs — every algorithm written
// before knobs existed. Tests that are not about knobs say so with this.
func noStrategyParameters(t *testing.T) domains.StrategyParametersDomain {
	t.Helper()

	parameters, buildError := domains.NewStrategyParametersDomain(nil)
	require.NoError(t, buildError)

	return parameters
}

const parameterisedAverageScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	period := indicator.LookbackCount("期數")
	factor := indicator.Number("倍數")

	sum := 0.0
	for _, candle := range data[len(data)-period:] {
		sum += candle.Close
	}

	return map[string]float64{"ma": sum / float64(period) * factor}
}
`

const booleanSwitchScript = `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	useOldest := indicator.Boolean("看最舊那一根")

	if useOldest {
		return map[string]float64{"價": data[0].Close}
	}

	return map[string]float64{"價": data[len(data)-1].Close}
}
`

func parametersOf(t *testing.T, declared ...dto.StrategyParameterWriteDto) domains.StrategyParametersDomain {
	t.Helper()

	parameters, buildError := domains.NewStrategyParametersDomain(declared)
	require.NoError(t, buildError)

	return parameters
}

// 算式拿到的必須是它那一種該有的樣子：回看根數要能直接拿去切片，數值要能直接拿去乘。
// 拿到別的東西，算式就得自己判斷，而判斷失敗會變成算式崩潰。
func TestAScriptReadsItsParametersByName(t *testing.T) {
	parameters := parametersOf(t,
		dto.StrategyParameterWriteDto{Name: "期數", Kind: "lookbackCount", DefaultValue: 2},
		dto.StrategyParameterWriteDto{Name: "倍數", Kind: "number", DefaultValue: 2.5})

	indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
		t.Context(), parameterisedAverageScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100, 110, 120), parameters)

	require.NoError(t, err)
	// 最後兩根（110、120）的均價 115，乘上 2.5。
	assert.InDelta(t, 287.5, numberOf(indicatorValues, "ma"), 0.0001)
}

// 是非讀出來必須是 bool，才能直接寫進 if。給它一個數字，算式就得自己判斷「幾算是」，
// 而那個判斷會在每一支算式裡各寫一次、各寫得不一樣。
func TestAScriptReadsABooleanAsAYesOrNo(t *testing.T) {
	candles := candlesWithClosePrices(100, 110, 120)

	t.Run("是的時候走這一邊", func(t *testing.T) {
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
			t.Context(), booleanSwitchScript, resultTypeOf(t, "float"), candles,
			parametersOf(t, dto.StrategyParameterWriteDto{
				Name: "看最舊那一根", Kind: "boolean", DefaultValue: 1,
			}))

		require.NoError(t, err)
		assert.InDelta(t, 100.0, numberOf(indicatorValues, "價"), 0.0001)
	})

	t.Run("否的時候走另一邊", func(t *testing.T) {
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
			t.Context(), booleanSwitchScript, resultTypeOf(t, "float"), candles,
			parametersOf(t, dto.StrategyParameterWriteDto{
				Name: "看最舊那一根", Kind: "boolean", DefaultValue: 0,
			}))

		require.NoError(t, err)
		assert.InDelta(t, 120.0, numberOf(indicatorValues, "價"), 0.0001)
	})

	t.Run("名字對不上時一樣被指名，不會安靜地拿到否", func(t *testing.T) {
		// 否是一個合法的答案，所以「拿不到」絕不能長得跟「答案是否」一樣。
		_, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
			t.Context(), booleanSwitchScript, resultTypeOf(t, "float"), candles,
			parametersOf(t, dto.StrategyParameterWriteDto{
				Name: "看最舊", Kind: "boolean", DefaultValue: 1,
			}))

		missingName, isUndeclared := domains.UndeclaredParameterName(err)
		require.True(t, isUndeclared)
		assert.Equal(t, "看最舊那一根", missingName)
	})
}

// 這一條是整個切片最容易做錯的地方：把參數改了名卻忘了改算式，
// 是很容易犯、而且完全看不出來的錯。它必須被說成「名字對不上」，不是「你的算式壞了」。
func TestReachingForAParameterNobodyDeclaredBlamesTheName(t *testing.T) {
	parameters := parametersOf(t,
		dto.StrategyParameterWriteDto{Name: "週期", Kind: "lookbackCount", DefaultValue: 2},
		dto.StrategyParameterWriteDto{Name: "倍數", Kind: "number", DefaultValue: 2.5})

	_, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
		t.Context(), parameterisedAverageScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100, 110, 120), parameters)

	require.ErrorIs(t, err, domains.ErrIndicatorParameterNotDeclared)
	assert.NotErrorIs(t, err, domains.ErrIndicatorScriptFailed,
		"這不是算式壞了，是名字對不上")
	assert.Contains(t, err.Error(), "期數", "它必須指出是哪一個名字")
}

// 兩種讀法都要擋，不是只有回看根數那一種：一個數值的名字打錯，
// 同樣會讓算式拿到零然後算出一個看起來正常的答案。
func TestReachingForAnUndeclaredNumberBlamesTheNameToo(t *testing.T) {
	parameters := parametersOf(t,
		dto.StrategyParameterWriteDto{Name: "期數", Kind: "lookbackCount", DefaultValue: 2},
		dto.StrategyParameterWriteDto{Name: "係數", Kind: "number", DefaultValue: 2.5})

	_, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
		t.Context(), parameterisedAverageScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100, 110, 120), parameters)

	require.ErrorIs(t, err, domains.ErrIndicatorParameterNotDeclared)
	assert.Contains(t, err.Error(), "倍數")
}

// 沒有宣告任何參數的算式一如既往——這是每一支既有算式的樣子。
func TestAScriptThatReadsNoParametersIsUnaffected(t *testing.T) {
	indicatorValues, err := script.NewYaegiIndicatorScriptProxy(2*time.Second).Execute(
		t.Context(), averageCloseScript, resultTypeOf(t, "float"),
		candlesWithClosePrices(100, 110, 120), parametersOf(t))

	require.NoError(t, err)
	assert.InDelta(t, 110.0, numberOf(indicatorValues, "ma"), 0)
}

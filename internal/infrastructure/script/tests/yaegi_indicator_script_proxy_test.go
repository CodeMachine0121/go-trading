package script_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
	"github.com/stretchr/testify/assert"
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
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
			Execute(averageCloseScript, candlesWithClosePrices(100, 110, 120))

		assert.NoError(t, err)
		assert.Len(t, indicatorValues, 1)
		assert.Equal(t, 110.0, indicatorValues["ma"])
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
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
			Execute(highestAndLowestScript, candlesWithClosePrices(100, 110, 120))

		assert.NoError(t, err)
		assert.Len(t, indicatorValues, 2)
		assert.Equal(t, 120.0, indicatorValues["high"])
		assert.Equal(t, 100.0, indicatorValues["low"])
	})

	t.Run("produces an empty set when the script names nothing", func(t *testing.T) {
		emptyScript := `
package main

import "indicator"

func Calculate(data []indicator.KCandle) map[string]float64 {
	return map[string]float64{}
}
`
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
			Execute(emptyScript, candlesWithClosePrices(100))

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
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
			Execute(nothingScript, candlesWithClosePrices(100))

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
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
			Execute(repeatedNameScript, candlesWithClosePrices(100))

		assert.NoError(t, err)
		assert.Len(t, indicatorValues, 1)
		assert.Equal(t, 120.0, indicatorValues["ma"])
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
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().Execute(
			readEverythingScript,
			[]vo.KCandleVo{{Close: 110.5, High: 120.25, Volume: 11.5, OpenTimeUnixSeconds: 1700000000}})

		assert.NoError(t, err)
		assert.Equal(t, 110.5, indicatorValues["close"])
		assert.Equal(t, 120.25, indicatorValues["high"])
		assert.Equal(t, 11.5, indicatorValues["volume"])
		assert.Equal(t, 1700000000.0, indicatorValues["openTime"])
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
		indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
			Execute(squareRootScript, candlesWithClosePrices(144))

		assert.NoError(t, err)
		assert.Equal(t, 12.0, indicatorValues["root"])
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
			indicatorValues, err := script.NewYaegiIndicatorScriptProxy().
				Execute(testCase.script, candlesWithClosePrices(100))

			assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
			assert.Contains(t, err.Error(), testCase.expectedReason)
			assert.Nil(t, indicatorValues)
		})
	}
}

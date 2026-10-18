package script_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func contractBarsWithClosePrices(closePrices ...float64) []vo.ContractKCandleVo {
	contractKCandleVos := make([]vo.ContractKCandleVo, 0, len(closePrices))
	for index, closePrice := range closePrices {
		contractKCandleVos = append(contractKCandleVos, vo.ContractKCandleVo{
			KCandleVo: vo.KCandleVo{
				Symbol:              "BTCUSDT",
				OpenTimeUnixSeconds: int64(1000 + index*3600),
				Close:               closePrice,
			},
		})
	}

	return contractKCandleVos
}

// averageCloseOfContractBarsScript is the spot script with only the entry point's element type changed.
const averageCloseOfContractBarsScript = `
package main

import "indicator"

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	sum := 0.0
	for _, candle := range data {
		sum += candle.Close
	}
	return map[string]float64{"ma": sum / float64(len(data))}
}
`

func TestContractExecuteReadsTheSpotFiguresUnderTheirSpotNames(t *testing.T) {
	indicatorValues, err := contractIndicatorScriptProxy(2*time.Second).
		Execute(t.Context(), averageCloseOfContractBarsScript, resultTypeOf(t, "float"),
			contractBarsWithClosePrices(100, 110, 120), noStrategyScriptParameters(t))

	require.NoError(t, err)
	assert.Equal(t, 110.0, numberOf(indicatorValues, "ma"))
}

func TestContractExecuteReadsTheFiguresOnlyAContractBarCarries(t *testing.T) {
	contractBars := contractBarsWithClosePrices(100)
	contractBars[0].Mark = vo.PriceLineVo{High: 106}
	contractBars[0].PremiumIndex = vo.PriceLineVo{Low: -0.0009}
	contractBars[0].FundingRate = 0.0001
	contractBars[0].FundingSettledInBar = true
	contractBars[0].OpenInterest = 5000
	contractBars[0].TopTraderPositionLongShortRatio = 1.5
	contractBars[0].TradeCount = 12000
	readEverythingScript := `
package main

import "indicator"

func closeOf(candle indicator.KCandle) float64 { return candle.Close }

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	bar := data[0]
	settled := 0.0
	if bar.FundingSettledInBar {
		settled = 1
	}
	var markLine indicator.PriceLine = bar.Mark
	return map[string]float64{
		"markHigh":     markLine.High,
		"premiumLow":   bar.PremiumIndex.Low,
		"fundingRate":  bar.FundingRate,
		"settled":      settled,
		"openInterest": bar.OpenInterest,
		"topRatio":     bar.TopTraderPositionLongShortRatio,
		"tradeCount":   float64(bar.TradeCount),
		"spotClose":    closeOf(bar.KCandleVo),
	}
}
`

	indicatorValues, err := contractIndicatorScriptProxy(2*time.Second).
		Execute(t.Context(), readEverythingScript, resultTypeOf(t, "float"), contractBars, noStrategyScriptParameters(t))

	require.NoError(t, err)
	assert.Equal(t, 106.0, numberOf(indicatorValues, "markHigh"))
	assert.Equal(t, -0.0009, numberOf(indicatorValues, "premiumLow"))
	assert.Equal(t, 0.0001, numberOf(indicatorValues, "fundingRate"))
	assert.Equal(t, 1.0, numberOf(indicatorValues, "settled"))
	assert.Equal(t, 5000.0, numberOf(indicatorValues, "openInterest"))
	assert.Equal(t, 1.5, numberOf(indicatorValues, "topRatio"))
	assert.Equal(t, 12000.0, numberOf(indicatorValues, "tradeCount"))
	assert.Equal(t, 100.0, numberOf(indicatorValues, "spotClose"))
}

func TestContractExecuteSaysAnEntryPointWrittenForSpotCandlesIsWrittenWrong(t *testing.T) {
	indicatorValues, err := contractIndicatorScriptProxy(2*time.Second).
		Execute(t.Context(), averageCloseScript, resultTypeOf(t, "float"),
			contractBarsWithClosePrices(100), noStrategyScriptParameters(t))

	require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, err.Error(), "func Calculate(data []indicator.ContractKCandle)")
	assert.Nil(t, indicatorValues)
}

func TestContractExecuteNamesTheKnobNobodyDeclared(t *testing.T) {
	undeclaredKnobScript := `
package main

import "indicator"

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	return map[string]float64{"period": float64(indicator.LookbackCount("週期"))}
}
`

	_, err := contractIndicatorScriptProxy(2*time.Second).
		Execute(t.Context(), undeclaredKnobScript, resultTypeOf(t, "float"),
			contractBarsWithClosePrices(100), noStrategyScriptParameters(t))

	parameterName, isUndeclared := domains.UndeclaredParameterName(err)
	require.True(t, isUndeclared)
	assert.Equal(t, "週期", parameterName)
}

func TestContractExecuteGivesUpOnAScriptThatOutlivesItsAllowance(t *testing.T) {
	neverEndingContractScript := `
package main

import "indicator"

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	for {
	}
}
`

	_, err := contractIndicatorScriptProxy(300*time.Millisecond).
		Execute(t.Context(), neverEndingContractScript, resultTypeOf(t, "float"),
			contractBarsWithClosePrices(100), noStrategyScriptParameters(t))

	require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Contains(t, err.Error(), "未能算完")
}

func TestContractExecuteForEachCandleRunsOnceOverEveryGrowingStretch(t *testing.T) {
	perBarValues, err := contractIndicatorScriptProxy(2*time.Second).
		ExecuteForEachCandle(t.Context(), averageCloseOfContractBarsScript, resultTypeOf(t, "float"),
			contractBarsWithClosePrices(100, 110, 120), noStrategyScriptParameters(t), nil)

	require.NoError(t, err)
	require.Len(t, perBarValues, 3)
	assert.Equal(t, 100.0, numberOf(perBarValues[0], "ma"))
	assert.Equal(t, 105.0, numberOf(perBarValues[1], "ma"))
	assert.Equal(t, 110.0, numberOf(perBarValues[2], "ma"))
}

func TestContractExecuteForEachCandleReportsABrokenScriptBeforeAnyBar(t *testing.T) {
	perBarValues, err := contractIndicatorScriptProxy(2*time.Second).
		ExecuteForEachCandle(t.Context(), "this is not go", resultTypeOf(t, "float"),
			contractBarsWithClosePrices(100), noStrategyScriptParameters(t), nil)

	require.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	assert.Nil(t, perBarValues)
}

// footprintReadingScript names both footprint types, so it fails to compile unless the sandbox exports them.
const footprintReadingScript = `
package main

import "indicator"

func lowestBuy(footprint indicator.Footprint) float64 {
	var level indicator.PriceLevel = footprint.Levels[0]
	return level.BuyVolume
}

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	bar := data[0]
	return map[string]float64{
		"step":       bar.Footprint.PriceStep,
		"levels":     float64(len(bar.Footprint.Levels)),
		"lowestBuy":  lowestBuy(bar.Footprint),
		"topSell":    bar.Footprint.Levels[1].SellVolume,
		"topPrice":   bar.Footprint.Levels[1].Price,
		"trades":     float64(bar.Footprint.Levels[1].TradeCount),
		"delta":      bar.Footprint.Delta,
		"control":    bar.Footprint.PointOfControl,
	}
}
`

func TestAContractScriptReadsTheFootprintOfEachBar(t *testing.T) {
	bars := contractBarsWithClosePrices(100)
	bars[0].Footprint = vo.OrderFlowFootprintVo{
		PriceStep: 10,
		Levels: []vo.OrderFlowPriceLevelVo{
			{Price: 84150, BuyVolume: 2, SellVolume: 1, TradeCount: 2},
			{Price: 84160, BuyVolume: 3, SellVolume: 0.5, TradeCount: 4},
		},
		Delta:          3.5,
		PointOfControl: 84150,
	}

	indicatorValues, err := contractIndicatorScriptProxy(2*time.Second).
		Execute(t.Context(), footprintReadingScript, resultTypeOf(t, "float"), bars, noStrategyScriptParameters(t))

	require.NoError(t, err)
	assert.Equal(t, map[string]float64{
		"step": 10, "levels": 2, "lowestBuy": 2, "topSell": 0.5, "topPrice": 84160, "trades": 4,
		"delta": 3.5, "control": 84150,
	}, map[string]float64{
		"step": numberOf(indicatorValues, "step"), "levels": numberOf(indicatorValues, "levels"),
		"lowestBuy": numberOf(indicatorValues, "lowestBuy"), "topSell": numberOf(indicatorValues, "topSell"),
		"topPrice": numberOf(indicatorValues, "topPrice"), "trades": numberOf(indicatorValues, "trades"),
		"delta": numberOf(indicatorValues, "delta"), "control": numberOf(indicatorValues, "control"),
	})
}

func TestContractExecuteForEachCandleKeepsEachRunAsCheapOnALongReplay(t *testing.T) {
	if raceDetectorOn {
		t.Skip("the race detector slows every run too much for a time bound to mean anything")
	}
	const lastCloseScript = `
package main

import "indicator"

func Calculate(data []indicator.ContractKCandle) map[string]float64 {
	return map[string]float64{"close": data[len(data)-1].Close}
}
`
	// Roughly sixteen months of quarter hours; when every run cost more than the last, this took tens of seconds and outgrew the memory cap.
	const barCount = 47000
	closePrices := make([]float64, barCount)
	for barIndex := range closePrices {
		closePrices[barIndex] = float64(barIndex)
	}

	started := time.Now()
	perBarValues, err := contractIndicatorScriptProxy(2*time.Second).
		ExecuteForEachCandle(t.Context(), lastCloseScript, resultTypeOf(t, "float"),
			contractBarsWithClosePrices(closePrices...), noStrategyScriptParameters(t), nil)

	require.NoError(t, err)
	require.Len(t, perBarValues, barCount)
	assert.Equal(t, float64(barCount-1), numberOf(perBarValues[barCount-1], "close"))
	assert.Less(t, time.Since(started), 5*time.Second)
}

package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bar carries a real high and low because exit levels are judged against them; an open left at zero falls back to the close.
type bar struct {
	open  float64
	high  float64
	low   float64
	close float64
}

// replayWithExitsOf stakes everything from 10,000, so a first close of 100 buys exactly 100 units and prices read as percentages.
func replayWithExitsOf(
	t *testing.T,
	stopLossPercentage int64,
	takeProfitPercentage int64,
	bars []bar,
	signals ...vo.SignalVo,
) dto.BacktestResultDto {
	t.Helper()

	positionSizing, sizingError := domains.NewPositionSizingDomain("allIn", decimal.Zero)
	require.NoError(t, sizingError)

	exitLevels, exitLevelsError := domains.NewBacktestExitLevelsDomain(
		decimal.NewFromInt(stopLossPercentage), decimal.NewFromInt(takeProfitPercentage))
	require.NoError(t, exitLevelsError)

	inputKCandles := make([]vo.KCandleVo, 0, len(bars))
	for candleIndex, candleBar := range bars {
		open := candleBar.open
		if open == 0 {
			open = candleBar.close
		}
		inputKCandles = append(inputKCandles, vo.KCandleVo{
			Symbol:              "BTCUSDT",
			OpenTimeUnixSeconds: replayStart.Add(time.Duration(candleIndex) * time.Hour).Unix(),
			Open:                open,
			High:                candleBar.high,
			Low:                 candleBar.low,
			Close:               candleBar.close,
		})
	}

	return domains.NewBacktestSimulationDomain(
		decimal.NewFromInt(10000),
		domains.NewBacktestPositionTermsDomain(positionSizing, exitLevels,
			domains.BacktestTransactionCostsDomain{}),
		domains.BacktestFillTimingDomain{},
		inputKCandles, signalDomainsSaying(signals...)).ToDto()
}

// aDippingBar dips to 97 and closes at 101: a 2% stop under 100 triggers inside it, which a close-only reading would miss.
func aDippingBar() bar {
	return bar{high: 103, low: 97, close: 101}
}

func TestBacktestSimulationClosesALongAtItsStop(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{{high: 100, low: 100, close: 100}, aDippingBar()},
		buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	closedTrade := result.ClosedTrades[0]
	assert.Equal(t, "98", closedTrade.ExitPrice.String())
	assert.Equal(t, "-200", closedTrade.Profit.String())
	assert.Equal(t, string(vo.TradeExitReasonStopLoss), closedTrade.ExitReason)
	assert.Equal(t, "9800", result.Summary.FinalEquity.String())
	assert.Equal(t, 1, result.Summary.StopLossExitCount)
	assert.Equal(t, 0, result.Summary.TakeProfitExitCount)
}

func TestBacktestSimulationReportsTwelvePercentLessWithAStopThanWithout(t *testing.T) {
	bars := []bar{
		{high: 100, low: 100, close: 100},
		aDippingBar(),
		{high: 110, low: 105, close: 110},
	}

	withoutAStop := replayWithExitsOf(t, 0, 0, bars, buySignal, holdSignal, holdSignal)
	// Held to the end: 100 units at 110.
	assert.Equal(t, "11000", withoutAStop.Summary.FinalEquity.String())
	assert.Empty(t, withoutAStop.ClosedTrades)

	withAStop := replayWithExitsOf(t, 2, 0, bars, buySignal, holdSignal, holdSignal)
	// Stopped out at 98 before the rally.
	assert.Equal(t, "9800", withAStop.Summary.FinalEquity.String())
}

func TestBacktestSimulationTreatsTouchingTheLevelAsReachingIt(t *testing.T) {
	testCases := []struct {
		name                string
		secondBar           bar
		expectedFinalEquity string
		expectedExitCount   int
	}{
		{
			name:                "a low of exactly the stop closes the position",
			secondBar:           bar{high: 103, low: 98, close: 101},
			expectedFinalEquity: "9800",
			expectedExitCount:   1,
		},
		{
			name:                "a low one above it does not",
			secondBar:           bar{high: 103, low: 99, close: 101},
			expectedFinalEquity: "10100",
			expectedExitCount:   0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := replayWithExitsOf(t, 2, 0,
				[]bar{{high: 100, low: 100, close: 100}, testCase.secondBar},
				buySignal, holdSignal)

			assert.Equal(t, testCase.expectedFinalEquity, result.Summary.FinalEquity.String())
			assert.Equal(t, testCase.expectedExitCount, result.Summary.StopLossExitCount)
		})
	}
}

func TestBacktestSimulationClosesALongAtItsTarget(t *testing.T) {
	result := replayWithExitsOf(t, 0, 5,
		[]bar{{high: 100, low: 100, close: 100}, {high: 106, low: 99, close: 104}},
		buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "105", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, string(vo.TradeExitReasonTakeProfit), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "10500", result.Summary.FinalEquity.String())
	assert.Equal(t, 1, result.Summary.TakeProfitExitCount)
}

// A candle's high and low don't say which came first, so the stop wins as the reading that never flatters the strategy.
func TestBacktestSimulationCountsACandleReachingBothLevelsAsAStop(t *testing.T) {
	result := replayWithExitsOf(t, 2, 5,
		[]bar{{high: 100, low: 100, close: 100}, {high: 106, low: 97, close: 104}},
		buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "98", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, string(vo.TradeExitReasonStopLoss), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "9800", result.Summary.FinalEquity.String())
}

func TestBacktestSimulationHonoursTheLevelsOnItsOneSide(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{{high: 100, low: 100, close: 100}, aDippingBar()},
		buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "long", result.ClosedTrades[0].Direction)
	assert.Equal(t, "98", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "9800", result.Summary.FinalEquity.String())
}

// The walk checks levels before applying the signal, so the entry candle's own low can't stop out the new position.
func TestBacktestSimulationNeverStopsOutOnTheEntryCandle(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{{high: 104, low: 96, close: 100}, {high: 110, low: 105, close: 110}},
		buySignal, holdSignal)

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, 0, result.Summary.StopLossExitCount)
	assert.Equal(t, "11000", result.Summary.FinalEquity.String())
}

// The close comes after an intrabar stop, so that bar's signal still applies.
func TestBacktestSimulationStillAppliesTheSignalOnTheCandleThatStoppedItOut(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{
			{high: 100, low: 100, close: 100},
			// Stopped out at 98, then bought again at this bar's close of 101.
			{high: 103, low: 97, close: 101},
			{high: 112, low: 110, close: 111},
		},
		buySignal, buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonStopLoss), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, 2, result.Summary.PositionOpenCount)
	// 9,800 buys 9800/101 units at 101, worth 10,770.30 at 111; rounded because the exact figure recurs.
	assert.Equal(t, "10770.3", result.Summary.FinalEquity.Round(2).String())
}

// A reopened position's stop is measured from its own entry (101), not the previous one (100).
func TestBacktestSimulationMeasuresAReopenedPositionsStopFromItsOwnEntry(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{
			{high: 100, low: 100, close: 100},
			{high: 103, low: 97, close: 101},
			// 98.98 is 2% under 101: this low clears the first stop of 98 but not the reopened one.
			{high: 102, low: 98.5, close: 100},
		},
		buySignal, buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 2)
	assert.Equal(t, "98", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "98.98", result.ClosedTrades[1].ExitPrice.String())
	assert.Equal(t, 2, result.Summary.StopLossExitCount)
}

func TestBacktestSimulationCallsEverySignalledExitWhatItIs(t *testing.T) {
	result := replayWithExitsOf(t, 0, 0,
		[]bar{
			{high: 100, low: 100, close: 100},
			{high: 110, low: 100, close: 110},
		},
		buySignal, sellSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonSignal), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, 0, result.Summary.StopLossExitCount)
	assert.Equal(t, 0, result.Summary.TakeProfitExitCount)
}

// A position exits only by signal, stop or target; nothing borrows, so there is no margin-call exit.
func TestBacktestSimulationEndsEveryTradeOneOfThreeWays(t *testing.T) {
	result := replayWithExitsOf(t, 2, 5,
		[]bar{
			{high: 100, low: 100, close: 100},
			aDippingBar(),
			{high: 100, low: 100, close: 100},
			{high: 106, low: 99, close: 100},
			{high: 100, low: 100, close: 100},
			{high: 101, low: 99, close: 100},
		},
		buySignal, holdSignal, buySignal, holdSignal, buySignal, sellSignal)

	require.Len(t, result.ClosedTrades, 3)
	exitReasons := make([]string, 0, len(result.ClosedTrades))
	for _, closedTrade := range result.ClosedTrades {
		assert.Contains(t, []string{
			string(vo.TradeExitReasonSignal),
			string(vo.TradeExitReasonStopLoss),
			string(vo.TradeExitReasonTakeProfit),
		}, closedTrade.ExitReason)
		exitReasons = append(exitReasons, closedTrade.ExitReason)
	}

	// All three exits actually occurred in this walk.
	assert.ElementsMatch(t, []string{
		string(vo.TradeExitReasonStopLoss),
		string(vo.TradeExitReasonTakeProfit),
		string(vo.TradeExitReasonSignal),
	}, exitReasons)
}

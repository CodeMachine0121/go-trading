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

// bar is one candle with a real high and a low, because that is what the exit levels
// are judged against. The candles elsewhere in these tests carry only a close: a
// replay given no distances never looks at the other two.
type bar struct {
	high  float64
	low   float64
	close float64
}

// replayWithExitsOf walks the bars under the two distances, staking everything on
// every opening from ten thousand. Those choices make the arithmetic readable: a
// first bar closing at 100 buys exactly 100 units, so a price is a percentage and a
// percentage is a price.
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
		inputKCandles = append(inputKCandles, vo.KCandleVo{
			Symbol:              "BTCUSDT",
			OpenTimeUnixSeconds: replayStart.Add(time.Duration(candleIndex) * time.Hour).Unix(),
			High:                candleBar.high,
			Low:                 candleBar.low,
			Close:               candleBar.close,
		})
	}

	return domains.NewBacktestSimulationDomain(
		decimal.NewFromInt(10000),
		domains.NewBacktestPositionTermsDomain(positionSizing, exitLevels,
			domains.BacktestTransactionCostsDomain{}),
		inputKCandles, signalDomainsSaying(signals...)).ToDto()
}

// The bar that does all the work below: it dips to 97 and recovers to close at 101.
// A stop two percent under 100 is reached inside it and a close-only reading would
// carry the position straight through.
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

// The whole point of this slice, as one comparison: the same script over the same
// three bars, once with a stop and once without.
func TestBacktestSimulationReportsTwelvePercentLessWithAStopThanWithout(t *testing.T) {
	bars := []bar{
		{high: 100, low: 100, close: 100},
		aDippingBar(),
		{high: 110, low: 105, close: 110},
	}

	withoutAStop := replayWithExitsOf(t, 0, 0, bars, buySignal, holdSignal, holdSignal)
	// The bet is carried to the end: a hundred units at 110.
	assert.Equal(t, "11000", withoutAStop.Summary.FinalEquity.String())
	assert.Empty(t, withoutAStop.ClosedTrades)

	withAStop := replayWithExitsOf(t, 2, 0, bars, buySignal, holdSignal, holdSignal)
	// The second bar's low reaches 98 and the rally happens without it.
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

// A single candle's high and low cannot say which price came first. Of the two
// readings, only this one never flatters the strategy.
func TestBacktestSimulationCountsACandleReachingBothLevelsAsAStop(t *testing.T) {
	result := replayWithExitsOf(t, 2, 5,
		[]bar{{high: 100, low: 100, close: 100}, {high: 106, low: 97, close: 104}},
		buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "98", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, string(vo.TradeExitReasonStopLoss), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "9800", result.Summary.FinalEquity.String())
}

// A replay only ever goes long, so it only ever has the one arrangement of levels —
// the stop below the entry and the target above it.
func TestBacktestSimulationHonoursTheLevelsOnItsOneSide(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{{high: 100, low: 100, close: 100}, aDippingBar()},
		buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "long", result.ClosedTrades[0].Direction)
	assert.Equal(t, "98", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "9800", result.Summary.FinalEquity.String())
}

// The entry filled at that bar's close, and that bar's low had already happened by
// then. Nothing compares times to know this: the walk examines the levels before
// applying the signal, so the position does not yet exist when its own bar is read.
func TestBacktestSimulationNeverStopsOutOnTheEntryCandle(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{{high: 104, low: 96, close: 100}, {high: 110, low: 105, close: 110}},
		buySignal, holdSignal)

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, 0, result.Summary.StopLossExitCount)
	assert.Equal(t, "11000", result.Summary.FinalEquity.String())
}

// A stop is reached during the bar and the close comes after it, so the signal
// standing on that bar is still heard. Swallowing it would let one stop eat an entry
// that had nothing to do with it.
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
	// 9,800 in cash bought 9800/101 units at 101; at 111 they are worth 10,770.30.
	// Rounded, because the exact figure recurs and what this case is about is that
	// the second opening happened at all.
	assert.Equal(t, "10770.3", result.Summary.FinalEquity.Round(2).String())
}

// The reopened bet measures its own stop from the price it actually got, not from the
// one before it. Its stop is two percent under 101, not under 100.
func TestBacktestSimulationMeasuresAReopenedPositionsStopFromItsOwnEntry(t *testing.T) {
	result := replayWithExitsOf(t, 2, 0,
		[]bar{
			{high: 100, low: 100, close: 100},
			{high: 103, low: 97, close: 101},
			// 98.98 is two percent under 101; a low of 99 would have survived the
			// first position's stop of 98 and does not survive this one.
			{high: 102, low: 98.5, close: 100},
		},
		buySignal, buySignal, holdSignal)

	require.Len(t, result.ClosedTrades, 2)
	assert.Equal(t, "98", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "98.98", result.ClosedTrades[1].ExitPrice.String())
	assert.Equal(t, 2, result.Summary.StopLossExitCount)
}

// Every trade a replay given no distances produces ends the one way it ever could.
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

// Three ways out and no fourth. A position is never taken off because the money
// behind it ran out — nothing here borrows, so there is nobody to call a loan in.
//
// Asserted over a walk that exits every way it can, rather than by reading the set of
// spellings: what matters is that no replay ever produces a fourth one.
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

	// All three really happened, so this is not passing on a walk that only ever
	// exited one way.
	assert.ElementsMatch(t, []string{
		string(vo.TradeExitReasonStopLoss),
		string(vo.TradeExitReasonTakeProfit),
		string(vo.TradeExitReasonSignal),
	}, exitReasons)
}

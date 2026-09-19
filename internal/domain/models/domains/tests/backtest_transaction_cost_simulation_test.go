package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// costedReplaySpec is one replay written the way the requirements talk about it, so
// the numbers below can be read against the specification without decoding six
// positional arguments.
//
// Most cases start from 10100 with everything staked and both rates at one percent.
// That is not a round number by accident: it makes the ceiling exactly 10000, so the
// first candle closing at 100 buys exactly 100 units and every figure afterwards can
// be checked in the head.
type costedReplaySpec struct {
	initialCapital      string
	tradingMode         string
	sizingMode          string
	sizingValue         string
	entryCostPercentage string
	exitCostPercentage  string
	closePrices         []float64
	signals             []vo.SignalVo
}

func (costedReplaySpec costedReplaySpec) run(t *testing.T) dto.BacktestResultDto {
	t.Helper()

	positionSizing, sizingError := domains.NewPositionSizingDomain(
		costedReplaySpec.sizingMode,
		decimal.RequireFromString(costedReplaySpec.sizingValue))
	require.NoError(t, sizingError)

	transactionCosts, costsError := domains.NewBacktestTransactionCostsDomain(
		decimal.RequireFromString(costedReplaySpec.entryCostPercentage),
		decimal.RequireFromString(costedReplaySpec.exitCostPercentage))
	require.NoError(t, costsError)

	inputKCandles := make([]vo.KCandleVo, 0, len(costedReplaySpec.closePrices))
	for candleIndex, closePrice := range costedReplaySpec.closePrices {
		inputKCandles = append(inputKCandles, replayedCandleAt(candleIndex, closePrice))
	}

	return domains.NewBacktestSimulationDomain(
		decimal.RequireFromString(costedReplaySpec.initialCapital), positionSizing,
		tradingModeOf(t, costedReplaySpec.tradingMode),
		domains.BacktestExitLevelsDomain{}, transactionCosts,
		inputKCandles, signalDomainsSaying(costedReplaySpec.signals...)).ToDto()
}

// aStakedSpotReplay is the shared setup: spot, everything staked, 10100 on hand.
// Spot rather than long-short because a sell there returns to cash, which is what
// makes a single round trip readable — a long-short sell would reverse into a short
// and the figures afterwards would belong to a second trade.
func aStakedSpotReplay(
	entryCostPercentage string, exitCostPercentage string,
	closePrices []float64, signals ...vo.SignalVo,
) costedReplaySpec {
	return costedReplaySpec{
		initialCapital:      "10100",
		tradingMode:         "spot",
		sizingMode:          "allIn",
		sizingValue:         "0",
		entryCostPercentage: entryCostPercentage,
		exitCostPercentage:  exitCostPercentage,
		closePrices:         closePrices,
		signals:             signals,
	}
}

// The whole slice as one comparison: the same script over the same three bars, once
// paying what a broker charges and once not.
func TestBacktestSimulationChargesBothEndsOfARoundTrip(t *testing.T) {
	closePrices := []float64{100, 105, 110}
	signals := []vo.SignalVo{buySignal, holdSignal, sellSignal}

	t.Run("paying one percent at each end", func(t *testing.T) {
		result := aStakedSpotReplay("1", "1", closePrices, signals...).run(t)

		require.Len(t, result.ClosedTrades, 1)
		closedTrade := result.ClosedTrades[0]
		assert.Equal(t, "100", closedTrade.EntryCost.String())
		assert.Equal(t, "110", closedTrade.ExitCost.String())
		assert.Equal(t, "790", closedTrade.Profit.String())
		assert.Equal(t, "10890", result.Summary.FinalEquity.String())
		assert.Equal(t, "210", result.Summary.TotalTransactionCost.String())
	})

	t.Run("naming no rates leaves every figure as it was", func(t *testing.T) {
		result := aStakedSpotReplay("0", "0", closePrices, signals...).run(t)

		require.Len(t, result.ClosedTrades, 1)
		closedTrade := result.ClosedTrades[0]
		assert.Equal(t, "0", closedTrade.EntryCost.String())
		assert.Equal(t, "0", closedTrade.ExitCost.String())
		assert.Equal(t, "1010", closedTrade.Profit.String())
		assert.Equal(t, "11110", result.Summary.FinalEquity.String())
		assert.Equal(t, "0", result.Summary.TotalTransactionCost.String())
	})
}

// A round trip that ends where it started is not a round trip that cost nothing.
func TestBacktestSimulationLosesExactlyTheTwoChargesOnAFlatRoundTrip(t *testing.T) {
	result := aStakedSpotReplay("1", "1",
		[]float64{100, 100, 100}, buySignal, holdSignal, sellSignal).run(t)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "-200", result.ClosedTrades[0].Profit.String())
	assert.Equal(t, "9900", result.Summary.FinalEquity.String())

	require.NotNil(t, result.Summary.WinRate)
	assert.InDelta(t, 0.0, *result.Summary.WinRate, 1e-9)
}

// The charge on the way out is taken on the money that changed hands, not on what the
// bet was worth. For a short those are different numbers, and reading the wrong one is
// a mistake that shows up on no long and looks perfectly plausible on the page.
//
// Closing a short reverses into a long — that is what long-short means — so the
// account figures afterwards belong to the next trade. What this case is about lives
// entirely on the round trip that finished.
func TestBacktestSimulationChargesAShortOnWhatChangedHands(t *testing.T) {
	testCases := []struct {
		name               string
		closePrices        []float64
		expectedExitCost   string
		expectedProfit     string
		costIfReadOffValue string
	}{
		{
			name:               "a winning short leaves at 90",
			closePrices:        []float64{100, 95, 90},
			expectedExitCost:   "90",
			expectedProfit:     "810",
			costIfReadOffValue: "110",
		},
		{
			name:               "a losing short leaves at 110",
			closePrices:        []float64{100, 105, 110},
			expectedExitCost:   "110",
			expectedProfit:     "-1210",
			costIfReadOffValue: "90",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := costedReplaySpec{
				initialCapital:      "10100",
				tradingMode:         "longShort",
				sizingMode:          "allIn",
				sizingValue:         "0",
				entryCostPercentage: "1",
				exitCostPercentage:  "1",
				closePrices:         testCase.closePrices,
				signals:             []vo.SignalVo{sellSignal, holdSignal, buySignal},
			}.run(t)

			require.Len(t, result.ClosedTrades, 1)
			closedTrade := result.ClosedTrades[0]
			assert.Equal(t, "100", closedTrade.EntryCost.String())
			assert.Equal(t, testCase.expectedExitCost, closedTrade.ExitCost.String())
			assert.NotEqual(t, testCase.costIfReadOffValue, closedTrade.ExitCost.String(),
				"the charge was taken on what the position was worth, not on what changed hands")
			assert.Equal(t, testCase.expectedProfit, closedTrade.Profit.String())
		})
	}
}

// Staking everything means the cash becomes the trade, not the position. The position
// ends up a little smaller and nothing is left owing.
func TestBacktestSimulationSplitsTheCashBetweenTheStakeAndItsCharge(t *testing.T) {
	testCases := []struct {
		name                string
		sizingMode          string
		sizingValue         string
		entryCostPercentage string
		expectedFirstEquity string
	}{
		{
			name:                "staking everything leaves exactly the charge behind",
			sizingMode:          "allIn",
			sizingValue:         "0",
			entryCostPercentage: "1",
			expectedFirstEquity: "10000",
		},
		{
			name:                "staking half still stakes half, and the charge comes on top",
			sizingMode:          "percentage",
			sizingValue:         "50",
			entryCostPercentage: "1",
			expectedFirstEquity: "10049.5",
		},
		{
			name:                "a charge of a hundred percent puts half in the position",
			sizingMode:          "allIn",
			sizingValue:         "0",
			entryCostPercentage: "100",
			expectedFirstEquity: "5050",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := costedReplaySpec{
				initialCapital:      "10100",
				tradingMode:         "spot",
				sizingMode:          testCase.sizingMode,
				sizingValue:         testCase.sizingValue,
				entryCostPercentage: testCase.entryCostPercentage,
				exitCostPercentage:  "0",
				closePrices:         []float64{100, 100},
				signals:             []vo.SignalVo{buySignal, holdSignal},
			}.run(t)

			assert.Equal(t, 1, result.Summary.PositionOpenCount)
			require.NotEmpty(t, result.EquityCurve)
			assert.Equal(t, testCase.expectedFirstEquity,
				result.EquityCurve[0].Equity.String())
		})
	}
}

// Affording an opening now means affording the charge that comes with it. The replay
// carries on flat, exactly as it always has when the cash fell short.
func TestBacktestSimulationSkipsAnOpeningThatCannotPayItsOwnCharge(t *testing.T) {
	testCases := []struct {
		name                      string
		sizingMode                string
		sizingValue               string
		entryCostPercentage       string
		expectedPositionOpenCount int
	}{
		{
			name:                      "a fixed amount equal to the cash no longer fits",
			sizingMode:                "fixedAmount",
			sizingValue:               "10000",
			entryCostPercentage:       "1",
			expectedPositionOpenCount: 0,
		},
		{
			name:                      "the same amount fits when trading is free",
			sizingMode:                "fixedAmount",
			sizingValue:               "10000",
			entryCostPercentage:       "0",
			expectedPositionOpenCount: 1,
		},
		{
			name:                      "staking a hundred percent is a figure, and it no longer fits",
			sizingMode:                "percentage",
			sizingValue:               "100",
			entryCostPercentage:       "1",
			expectedPositionOpenCount: 0,
		},
		{
			name:                      "staking everything adapts instead, because it names no figure",
			sizingMode:                "allIn",
			sizingValue:               "0",
			entryCostPercentage:       "1",
			expectedPositionOpenCount: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := costedReplaySpec{
				initialCapital:      "10000",
				tradingMode:         "spot",
				sizingMode:          testCase.sizingMode,
				sizingValue:         testCase.sizingValue,
				entryCostPercentage: testCase.entryCostPercentage,
				exitCostPercentage:  "0",
				closePrices:         []float64{100, 100},
				signals:             []vo.SignalVo{buySignal, holdSignal},
			}.run(t)

			assert.Equal(t, testCase.expectedPositionOpenCount,
				result.Summary.PositionOpenCount)
		})
	}
}

// The reason the profit is net, stated as the only number anybody reads: three round
// trips that all moved the right way, two of which did not move far enough to cover
// what they cost.
func TestBacktestSimulationWinRateCountsOnlyRoundTripsThatBeatTheirOwnCharges(t *testing.T) {
	scalpingBars := []float64{100, 101, 100, 101, 100, 105}
	scalpingSignals := []vo.SignalVo{
		buySignal, sellSignal, buySignal, sellSignal, buySignal, sellSignal}

	scalping := costedReplaySpec{
		initialCapital: "100000",
		tradingMode:    "spot",
		sizingMode:     "fixedAmount",
		sizingValue:    "10000",
		closePrices:    scalpingBars,
		signals:        scalpingSignals,
	}

	t.Run("paying one percent at each end", func(t *testing.T) {
		costed := scalping
		costed.entryCostPercentage = "1"
		costed.exitCostPercentage = "1"

		result := costed.run(t)

		require.Len(t, result.ClosedTrades, 3)
		assert.Equal(t, "-101", result.ClosedTrades[0].Profit.String())
		assert.Equal(t, "-101", result.ClosedTrades[1].Profit.String())
		assert.Equal(t, "295", result.ClosedTrades[2].Profit.String())

		require.NotNil(t, result.Summary.WinRate)
		assert.InDelta(t, 1.0/3.0, *result.Summary.WinRate, 1e-9)
		assert.Equal(t, "607", result.Summary.TotalTransactionCost.String())
		assert.Equal(t, "100093", result.Summary.FinalEquity.String())
	})

	t.Run("the same three round trips with trading free", func(t *testing.T) {
		free := scalping
		free.entryCostPercentage = "0"
		free.exitCostPercentage = "0"

		result := free.run(t)

		require.NotNil(t, result.Summary.WinRate)
		assert.InDelta(t, 1.0, *result.Summary.WinRate, 1e-9,
			"every one of them gained, and gross that is all a win rate can see")
		assert.Equal(t, "100700", result.Summary.FinalEquity.String())
	})
}

// A position still open has paid to get in and has not paid to get out. Reporting the
// second would describe something that has not happened; the price of not reporting it
// is a final equity one exit charge too kind, which the report card says out loud.
func TestBacktestSimulationNeverPreChargesAPositionStillOpen(t *testing.T) {
	result := aStakedSpotReplay("1", "1",
		[]float64{100, 110}, buySignal, holdSignal).run(t)

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, "11000", result.Summary.FinalEquity.String())
	assert.Equal(t, "100", result.Summary.TotalTransactionCost.String())
}

// The charge for opening is money already gone, so the curve drops on the candle that
// paid it — and the worst fall along the way grows accordingly. That is not noise in
// the measurement; it is the measurement.
func TestBacktestSimulationDrawsTheEntryChargeOnTheCurveAtOnce(t *testing.T) {
	closePrices := []float64{100, 110}
	signals := []vo.SignalVo{buySignal, holdSignal}

	costed := aStakedSpotReplay("1", "1", closePrices, signals...).run(t)
	free := aStakedSpotReplay("0", "0", closePrices, signals...).run(t)

	require.NotEmpty(t, costed.EquityCurve)
	assert.Equal(t, "10000", costed.EquityCurve[0].Equity.String())
	assert.Equal(t, "10100", free.EquityCurve[0].Equity.String())
	assert.Greater(t, costed.Summary.MaximumDrawdown, free.Summary.MaximumDrawdown)
}

// Two candles' worth of Taiwan's real numbers, as a guard on the shape of the thing:
// asymmetric rates, neither of them round, and a position that pays both ends.
func TestBacktestSimulationChargesTaiwanRatesAsymmetrically(t *testing.T) {
	result := costedReplaySpec{
		initialCapital:      "1000000",
		tradingMode:         "spot",
		sizingMode:          "fixedAmount",
		sizingValue:         "100000",
		entryCostPercentage: "0.0855",
		exitCostPercentage:  "0.3855",
		closePrices:         []float64{100, 100},
		signals:             []vo.SignalVo{buySignal, sellSignal},
	}.run(t)

	require.Len(t, result.ClosedTrades, 1)
	closedTrade := result.ClosedTrades[0]
	assert.Equal(t, "85.5", closedTrade.EntryCost.String())
	assert.Equal(t, "385.5", closedTrade.ExitCost.String())
	assert.Equal(t, "-471", closedTrade.Profit.String(),
		"a Taiwan round trip that goes nowhere costs 0.471% of the stake")
	assert.Equal(t, "471", result.Summary.TotalTransactionCost.String())
}

// Naming only one rate charges it at both ends. The two boxes exist so asymmetry can
// be said, not so symmetry has to be said twice.
func TestBacktestSimulationChargesTheEntryRateOnTheWayOutWhenNoneWasNamed(t *testing.T) {
	named := aStakedSpotReplay("1", "1",
		[]float64{100, 105, 110}, buySignal, holdSignal, sellSignal).run(t)
	inherited := aStakedSpotReplay("1", "0",
		[]float64{100, 105, 110}, buySignal, holdSignal, sellSignal).run(t)

	require.Len(t, inherited.ClosedTrades, 1)
	assert.Equal(t, named.ClosedTrades[0].ExitCost.String(),
		inherited.ClosedTrades[0].ExitCost.String())
	assert.Equal(t, named.Summary.FinalEquity.String(),
		inherited.Summary.FinalEquity.String())
}

// A replay handed nothing behaves as it did before any of this existed. The suite
// around this file is the real guard — not one assertion in it changed — and this is
// the statement of intent beside it.
func TestBacktestSimulationChargesNothingWhenNoRatesAreNamed(t *testing.T) {
	transactionCosts, buildError := domains.NewBacktestTransactionCostsDomain(
		decimal.Zero, decimal.Zero)
	require.NoError(t, buildError)

	assert.Equal(t, "10100",
		transactionCosts.MaximumStakeFrom(decimal.NewFromInt(10100)).String())
	assert.True(t, transactionCosts.EntryCostFor(decimal.NewFromInt(10100)).IsZero())
	assert.True(t, transactionCosts.ExitCostFor(decimal.NewFromInt(10100)).IsZero())
}

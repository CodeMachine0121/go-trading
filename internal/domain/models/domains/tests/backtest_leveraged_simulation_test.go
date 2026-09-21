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

// leveragedReplaySpec is one borrowed replay, written out. Every figure a case cares
// about is here and nothing else: the ones left blank are the ones that make no
// difference to what is being shown.
type leveragedReplaySpec struct {
	initialCapital        string
	sizingMode            string
	sizingValue           string
	tradingMode           string
	multiplier            string
	maintenanceMarginRate string
	stopLossPercentage    string
	takeProfitPercentage  string
	entryCostPercentage   string
	bars                  []bar
	signals               []vo.SignalVo
}

// leveragedReplayOf walks the bars under those settings. Bars carry a real high and a
// low because that is what any forced exit is judged against.
func leveragedReplayOf(t *testing.T, spec leveragedReplaySpec) dto.BacktestResultDto {
	t.Helper()

	positionSizing, sizingError := domains.NewPositionSizingDomain(
		spec.sizingMode, decimalOrZeroFromString(t, spec.sizingValue))
	require.NoError(t, sizingError)

	tradingMode := tradingModeOf(t, spec.tradingMode)

	exitLevels, exitLevelsError := domains.NewBacktestExitLevelsDomain(
		decimalOrZeroFromString(t, spec.stopLossPercentage),
		decimalOrZeroFromString(t, spec.takeProfitPercentage))
	require.NoError(t, exitLevelsError)

	leverage, leverageError := domains.NewBacktestLeverageDomain(
		decimalOrZeroFromString(t, spec.multiplier),
		decimalOrZeroFromString(t, spec.maintenanceMarginRate),
		tradingMode)
	require.NoError(t, leverageError)

	transactionCosts, costsError := domains.NewBacktestTransactionCostsDomain(
		decimalOrZeroFromString(t, spec.entryCostPercentage), decimal.Zero)
	require.NoError(t, costsError)

	inputKCandles := make([]vo.KCandleVo, 0, len(spec.bars))
	for candleIndex, candleBar := range spec.bars {
		inputKCandles = append(inputKCandles, vo.KCandleVo{
			Symbol:              "BTCUSDT",
			OpenTimeUnixSeconds: replayStart.Add(time.Duration(candleIndex) * time.Hour).Unix(),
			High:                candleBar.high,
			Low:                 candleBar.low,
			Close:               candleBar.close,
		})
	}

	return domains.NewBacktestSimulationDomain(
		decimal.RequireFromString(spec.initialCapital), tradingMode,
		domains.NewBacktestPositionTermsDomain(
			positionSizing, exitLevels, leverage, transactionCosts),
		inputKCandles, signalDomainsSaying(spec.signals...)).ToDto()
}

func decimalOrZeroFromString(t *testing.T, figure string) decimal.Decimal {
	t.Helper()

	if figure == "" {
		return decimal.Zero
	}

	return decimal.RequireFromString(figure)
}

// anEntryBar is the first bar of every case below: it neither rises nor falls, so the
// position is opened at exactly 100 and a price reads as a percentage.
func anEntryBar() bar {
	return bar{high: 100, low: 100, close: 100}
}

// aBorrowedLongReplay stakes everything at five times from ten thousand, so the
// position holds 500 units and every figure below is a round number.
func aBorrowedLongReplay(secondBar bar) leveragedReplaySpec {
	return leveragedReplaySpec{
		initialCapital: "10000", sizingMode: "allIn", tradingMode: "longShort",
		multiplier: "5", maintenanceMarginRate: "0.5",
		bars:    []bar{anEntryBar(), secondBar},
		signals: []vo.SignalVo{buySignal, holdSignal},
	}
}

// What a borrowed position is worth: the market moves the exposure, not the stake.
func TestLeveragedReplayMovesTheExposureRatherThanTheStake(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedLongReplay(bar{high: 110, low: 100, close: 110}))

	// Five times ten thousand at 100 is 500 units; ten points of those is 5000.
	assert.Equal(t, "15000", result.Summary.FinalEquity.String())
	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
}

func TestLeveragedReplayLosesTheExposureJustAsFast(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedLongReplay(bar{high: 100, low: 90, close: 90}))

	assert.Equal(t, "5000", result.Summary.FinalEquity.String())
	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
}

// The control. Borrowing nothing leaves the same walk exactly where it was.
func TestUnleveragedReplayIsUntouched(t *testing.T) {
	spec := aBorrowedLongReplay(bar{high: 110, low: 100, close: 110})
	spec.multiplier = ""
	spec.maintenanceMarginRate = ""

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, "11000", result.Summary.FinalEquity.String())
	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
}

// A rate declared with nothing borrowed changes nothing, which is what lets the two
// figures be one optional group rather than two that have to agree.
func TestUnleveragedReplayIgnoresAMaintenanceMarginRate(t *testing.T) {
	spec := aBorrowedLongReplay(bar{high: 110, low: 100, close: 110})
	spec.multiplier = ""
	spec.maintenanceMarginRate = "50"

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, "11000", result.Summary.FinalEquity.String())
}

// The whole point of the slice: the account is emptied and the report card says so.
func TestLeveragedReplayIsWipedOutWhenTheLoanIsCalledIn(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedLongReplay(bar{high: 100, low: 80, close: 90}))

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonLiquidation), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "80.5", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "-10000", result.ClosedTrades[0].Profit.String())
	assert.Equal(t, "0", result.Summary.FinalEquity.String())
	assert.Equal(t, 1, result.Summary.LiquidationExitCount)
	assert.Equal(t, 1, result.Summary.PositionOpenCount)
}

func TestLeveragedReplayCountsTouchingTheLiquidationPriceAsReachingIt(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedLongReplay(bar{high: 100, low: 80.5, close: 90}))

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonLiquidation), result.ClosedTrades[0].ExitReason)
}

func TestLeveragedReplayHoldsOnJustShortOfTheLiquidationPrice(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedLongReplay(bar{high: 100, low: 80.6, close: 81}))

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
	// Still open at the end, valued at the last close: 500 units nineteen points down.
	assert.Equal(t, "500", result.Summary.FinalEquity.String())
}

// A bar that gaps straight through. The exit still fills where the loan was called
// in, and the account cannot end up owing anything.
func TestLeveragedReplayCannotLoseMoreThanTheStake(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedLongReplay(bar{high: 100, low: 10, close: 10}))

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "80.5", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "-10000", result.ClosedTrades[0].Profit.String())
	assert.Equal(t, "0", result.Summary.FinalEquity.String())
	assert.False(t, result.Summary.FinalEquity.IsNegative())
}

// A short is the mirror image: it is the price rising that empties it.
func TestLeveragedReplayWipesOutAShortOnTheWayUp(t *testing.T) {
	spec := aBorrowedLongReplay(bar{high: 120, low: 100, close: 115})
	spec.signals = []vo.SignalVo{sellSignal, holdSignal}

	result := leveragedReplayOf(t, spec)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonLiquidation), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "119.5", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "0", result.Summary.FinalEquity.String())
}

// The stop and the liquidation, racing on the same side of the entry. The nearer one
// wins every time, and one of the two outcomes is an account that survived.
func TestLeveragedReplayTakesWhicheverAdverseExitIsNearer(t *testing.T) {
	testCases := []struct {
		name                 string
		stopLossPercentage   string
		expectedExitReason   vo.TradeExitReasonVo
		expectedExitPrice    string
		expectedFinalEquity  string
		expectedStopCount    int
		expectedLiquidations int
	}{
		{
			name:               "a stop inside the liquidation price saves the account",
			stopLossPercentage: "5",
			expectedExitReason: vo.TradeExitReasonStopLoss,
			expectedExitPrice:  "95", expectedFinalEquity: "7500",
			expectedStopCount: 1, expectedLiquidations: 0,
		},
		{
			name:               "a stop beyond it never gets a turn",
			stopLossPercentage: "30",
			expectedExitReason: vo.TradeExitReasonLiquidation,
			expectedExitPrice:  "80.5", expectedFinalEquity: "0",
			expectedStopCount: 0, expectedLiquidations: 1,
		},
		{
			name:               "no stop at all leaves the loan to do it",
			stopLossPercentage: "",
			expectedExitReason: vo.TradeExitReasonLiquidation,
			expectedExitPrice:  "80.5", expectedFinalEquity: "0",
			expectedStopCount: 0, expectedLiquidations: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			spec := aBorrowedLongReplay(bar{high: 100, low: 60, close: 60})
			spec.stopLossPercentage = testCase.stopLossPercentage

			result := leveragedReplayOf(t, spec)

			require.Len(t, result.ClosedTrades, 1)
			assert.Equal(t,
				string(testCase.expectedExitReason), result.ClosedTrades[0].ExitReason)
			assert.Equal(t, testCase.expectedExitPrice, result.ClosedTrades[0].ExitPrice.String())
			assert.Equal(t, testCase.expectedFinalEquity, result.Summary.FinalEquity.String())
			// The two counts are the same fact read from either side: a run whose
			// stop is nearer can never be liquidated, and one whose stop is farther
			// can never be stopped out. Both are asserted so neither bucket can
			// quietly collect the other's trades.
			assert.Equal(t, testCase.expectedStopCount, result.Summary.StopLossExitCount)
			assert.Equal(t, testCase.expectedLiquidations, result.Summary.LiquidationExitCount)
		})
	}
}

// A bar reaching both sides is still read the way it always was: the adverse side
// first, because a single bar cannot say which came first and only one reading never
// flatters the strategy.
func TestLeveragedReplayStillReadsTheAdverseSideFirst(t *testing.T) {
	spec := aBorrowedLongReplay(bar{high: 115, low: 80, close: 110})
	spec.stopLossPercentage = "5"
	spec.takeProfitPercentage = "10"

	result := leveragedReplayOf(t, spec)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonStopLoss), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "95", result.ClosedTrades[0].ExitPrice.String())
}

// What the venue charges follows the exposure, so a borrowed position pays its
// multiplier over. Ten thousand staked at four times against a half-percent charge
// leaves exactly nothing behind, which is what makes the arithmetic readable here.
func TestLeveragedReplayChargesTheExposureRatherThanTheStake(t *testing.T) {
	spec := leveragedReplaySpec{
		initialCapital: "10200", sizingMode: "allIn", tradingMode: "longShort",
		multiplier: "4", maintenanceMarginRate: "0.5", entryCostPercentage: "0.5",
		bars:    []bar{anEntryBar(), bar{high: 100, low: 100, close: 100}},
		signals: []vo.SignalVo{buySignal, holdSignal},
	}

	result := leveragedReplayOf(t, spec)

	// Staked 10000, exposed 40000, charged 200 — and the cash covered both exactly.
	assert.Equal(t, "200", result.Summary.TotalTransactionCost.String())
	assert.Equal(t, 1, result.Summary.PositionOpenCount)
	assert.Equal(t, "10000", result.Summary.FinalEquity.String())
}

// A fixed amount the cash covers on its own, but not once the borrowed charge is
// beside it. That is an opening skipped, not a failure.
func TestLeveragedReplaySkipsAnOpeningItCannotAffordTheChargeFor(t *testing.T) {
	spec := leveragedReplaySpec{
		initialCapital: "10000", sizingMode: "fixedAmount", sizingValue: "10000",
		tradingMode: "longShort", multiplier: "5", maintenanceMarginRate: "0.5",
		entryCostPercentage: "0.1",
		bars:                []bar{anEntryBar(), bar{high: 100, low: 100, close: 100}},
		signals:             []vo.SignalVo{buySignal, holdSignal},
	}

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, 0, result.Summary.PositionOpenCount)
	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, "10000", result.Summary.FinalEquity.String())
}

// The same fixed amount goes through once nothing is borrowed, which is what shows
// the multiplier is what made the difference rather than the rate alone.
func TestUnleveragedReplayAffordsTheSameFixedAmount(t *testing.T) {
	spec := leveragedReplaySpec{
		initialCapital: "10000", sizingMode: "fixedAmount", sizingValue: "9000",
		tradingMode: "longShort", entryCostPercentage: "0.1",
		bars:    []bar{anEntryBar(), bar{high: 100, low: 100, close: 100}},
		signals: []vo.SignalVo{buySignal, holdSignal},
	}

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, 1, result.Summary.PositionOpenCount)
	assert.Equal(t, "9", result.Summary.TotalTransactionCost.String())
}

// The charge at each end, read off one position rather than a whole walk, against the
// two figures the requirements state.
func TestBorrowedPositionChargesBothEndsOnTheExposure(t *testing.T) {
	transactionCosts, costsError := domains.NewBacktestTransactionCostsDomain(
		decimal.RequireFromString("0.1"), decimal.Zero)
	require.NoError(t, costsError)

	leverage, leverageError := leverageOf(t, "5", "0.5")
	require.NoError(t, leverageError)

	position, isOpened := positionTakenOn(
		t, vo.PositionDirectionLong,
		decimal.NewFromInt(100), decimal.NewFromInt(10000),
		domains.BacktestExitLevelsDomain{}, leverage, transactionCosts)
	require.True(t, isOpened)

	assert.Equal(t, "50", position.EntryCost().String())

	closedTrade := position.ClosedAt(
		positionExitTime, decimal.NewFromInt(110), vo.TradeExitReasonSignal)
	assert.Equal(t, "55", closedTrade.ExitCost.String())
}

func TestUnborrowedPositionChargesTheStakeAsItAlwaysDid(t *testing.T) {
	transactionCosts, costsError := domains.NewBacktestTransactionCostsDomain(
		decimal.RequireFromString("0.1"), decimal.Zero)
	require.NoError(t, costsError)

	position, isOpened := positionTakenOn(
		t, vo.PositionDirectionLong,
		decimal.NewFromInt(100), decimal.NewFromInt(10000),
		domains.BacktestExitLevelsDomain{}, domains.BacktestLeverageDomain{}, transactionCosts)
	require.True(t, isOpened)

	assert.Equal(t, "10", position.EntryCost().String())
}

// A loan called in takes the margin and the charge already paid with it, and charges
// nothing more on the way out — there is nothing left to pay it with.
func TestLiquidatedTradeLosesTheStakeAndTheChargeAlreadyPaid(t *testing.T) {
	transactionCosts, costsError := domains.NewBacktestTransactionCostsDomain(
		decimal.RequireFromString("0.5"), decimal.Zero)
	require.NoError(t, costsError)

	leverage, leverageError := leverageOf(t, "4", "0.5")
	require.NoError(t, leverageError)

	position, isOpened := positionTakenOn(
		t, vo.PositionDirectionLong,
		decimal.NewFromInt(100), decimal.NewFromInt(10000),
		domains.BacktestExitLevelsDomain{}, leverage, transactionCosts)
	require.True(t, isOpened)

	closedTrade := position.ClosedAt(
		positionExitTime, decimal.RequireFromString("75.5"), vo.TradeExitReasonLiquidation)

	assert.Equal(t, "200", closedTrade.EntryCost.String())
	assert.Equal(t, "0", closedTrade.ExitCost.String())
	assert.Equal(t, "-10200", closedTrade.Profit.String())
	assert.Equal(t, "0", position.CashReturnedFor(closedTrade).String())
}

// Every other way out still settles the way it always has.
func TestUnliquidatedTradeStillReturnsWhatItIsWorth(t *testing.T) {
	position, isOpened := positionTakenOn(
		t, vo.PositionDirectionLong,
		decimal.NewFromInt(100), decimal.NewFromInt(10000),
		domains.BacktestExitLevelsDomain{}, domains.BacktestLeverageDomain{},
		domains.BacktestTransactionCostsDomain{})
	require.True(t, isOpened)

	closedTrade := position.ClosedAt(
		positionExitTime, decimal.NewFromInt(110), vo.TradeExitReasonSignal)

	assert.Equal(t, "11000", position.CashReturnedFor(closedTrade).String())
}

// A position paid for in full can still cost more than it staked, and the account
// says so — exactly as it did before there was anything to borrow.
//
// This is the case a floor on the way out would have quietly rewritten: the report
// card would have bottomed out at zero while the trade went on reporting the real
// loss, and the difference would have appeared as money out of nowhere.
func TestAnUnborrowedPositionCanStillCostMoreThanItStaked(t *testing.T) {
	position, isOpened := positionTakenOn(
		t, vo.PositionDirectionShort,
		decimal.NewFromInt(100), decimal.NewFromInt(10000),
		domains.BacktestExitLevelsDomain{}, domains.BacktestLeverageDomain{},
		domains.BacktestTransactionCostsDomain{})
	require.True(t, isOpened)

	// A hundred units sold at 100 and bought back at 300 loses twice the stake.
	closedTrade := position.ClosedAt(
		positionExitTime, decimal.NewFromInt(300), vo.TradeExitReasonSignal)

	assert.Equal(t, "-20000", closedTrade.Profit.String())
	assert.Equal(t, "-10000", position.CashReturnedFor(closedTrade).String())
}

// The whole walk, so that the report card and the trade list are read together —
// that is where a floor would have shown up as a disagreement.
func TestAnUnborrowedReplayReportsGoingPastZero(t *testing.T) {
	spec := aBorrowedLongReplay(bar{high: 300, low: 100, close: 300})
	spec.multiplier = ""
	spec.maintenanceMarginRate = ""
	spec.signals = []vo.SignalVo{sellSignal, buySignal}

	result := leveragedReplayOf(t, spec)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, "-20000", result.ClosedTrades[0].Profit.String())
	// Ten thousand staked, twenty thousand lost. The account really is ten thousand
	// in the hole, and saying zero here would be inventing ten thousand.
	assert.Equal(t, "-10000", result.Summary.FinalEquity.String())
}

// A borrowed position is the other way round: the loan is called in for whatever it
// cannot cover, so the charge is capped at what the position was still worth and the
// margin is gone. Cash and the finished trade agree, which is the point.
func TestABorrowedPositionCannotPayAChargeItNoLongerHas(t *testing.T) {
	transactionCosts, costsError := domains.NewBacktestTransactionCostsDomain(
		decimal.Zero, decimal.RequireFromString("1"))
	require.NoError(t, costsError)

	exitLevels, exitLevelsError := domains.NewBacktestExitLevelsDomain(
		decimal.RequireFromString("4.4"), decimal.Zero)
	require.NoError(t, exitLevelsError)

	// Twenty times is liquidated at 4.5% under, so a stop at 4.4% takes it off
	// first — and one percent of the exposure on the way out is more than the
	// margin has left.
	leverage, leverageError := leverageOf(t, "20", "0.5")
	require.NoError(t, leverageError)

	position, isOpened := positionTakenOn(
		t, vo.PositionDirectionLong,
		decimal.NewFromInt(100), decimal.NewFromInt(10000),
		exitLevels, leverage, transactionCosts)
	require.True(t, isOpened)

	closedTrade := position.ClosedAt(
		positionExitTime, decimal.RequireFromString("95.6"), vo.TradeExitReasonStopLoss)

	// 2000 units four point four under is 8800 lost, leaving 1200 of the margin —
	// and the charge would have been 1912.
	assert.Equal(t, "1200", closedTrade.ExitCost.String())
	assert.Equal(t, "-10000", closedTrade.Profit.String())
	assert.Equal(t, "0", position.CashReturnedFor(closedTrade).String())
}

// The report card counts them apart, because "the stop kept catching it" and "this
// account was emptied three times" read identically without this figure.
func TestReportCardSeparatesLiquidationsFromStops(t *testing.T) {
	spec := leveragedReplaySpec{
		initialCapital: "10000", sizingMode: "allIn", tradingMode: "longShort",
		multiplier: "5", maintenanceMarginRate: "0.5",
		bars: []bar{
			anEntryBar(),
			{high: 100, low: 80, close: 85},
			{high: 85, low: 85, close: 85},
		},
		signals: []vo.SignalVo{buySignal, holdSignal, holdSignal},
	}

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, 1, result.Summary.LiquidationExitCount)
	assert.Equal(t, 0, result.Summary.StopLossExitCount)
	assert.Equal(t, 0, result.Summary.TakeProfitExitCount)
}

func TestReportCardCountsNoLiquidationsWithoutALoan(t *testing.T) {
	spec := aBorrowedLongReplay(bar{high: 100, low: 10, close: 10})
	spec.multiplier = ""
	spec.maintenanceMarginRate = ""

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
	assert.Empty(t, result.ClosedTrades)
}

// A percentage that was affordable yesterday can be refused today purely because
// somebody added leverage — the charge is levied on the exposure, so the multiplier
// multiplies it. The refusal has to name that, or the caller goes and lowers the one
// knob that was never the problem.
func TestRefusingAnUnstakeablePercentageNamesTheBorrowing(t *testing.T) {
	requestDto := dto.BacktestRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: replayStart, EndTime: replayStart.Add(10 * time.Hour),
		InitialCapital:      decimal.NewFromInt(10000),
		PositionSizingMode:  "percentage",
		PositionSizingValue: decimal.NewFromInt(95),
		EntryCostPercentage: decimal.RequireFromString("0.3"),
		Leverage:            decimal.NewFromInt(20),
	}

	_, buildError := domains.NewBacktestDomain(
		requestDto, 1000, replayStart.Add(20*time.Hour))

	require.Error(t, buildError)
	// Both halves, each by a phrase that appears nowhere else in the sentence: why
	// the multiplier did this, and that lowering it is a way out. "槓桿倍數" alone
	// is not enough — it appears twice, so either half could quietly disappear.
	assert.Contains(t, buildError.Error(), "進場成本是照**放大後的曝險金額**收的")
	assert.Contains(t, buildError.Error(), "調低槓桿倍數與調低這個百分比一樣有效")
	// Still pointed at the figure the caller types, because that is the box on the
	// screen — the sentence is what says which of the two to change.
	fieldName, namesField := domains.BacktestFieldName(buildError)
	require.True(t, namesField)
	assert.Equal(t, "positionSizingValue", fieldName)
}

// The same refusal without borrowing says nothing about it — there is nothing to say,
// and a sentence about leverage would send somebody looking for a box they left empty.
func TestRefusingAnUnstakeablePercentageStaysQuietWithoutBorrowing(t *testing.T) {
	requestDto := dto.BacktestRequestDto{
		Symbol: "BTCUSDT", AggregationInterval: "1h",
		StartTime: replayStart, EndTime: replayStart.Add(10 * time.Hour),
		InitialCapital:      decimal.NewFromInt(10000),
		PositionSizingMode:  "percentage",
		PositionSizingValue: decimal.NewFromInt(100),
		EntryCostPercentage: decimal.RequireFromString("0.3"),
	}

	_, buildError := domains.NewBacktestDomain(
		requestDto, 1000, replayStart.Add(20*time.Hour))

	require.Error(t, buildError)
	assert.NotContains(t, buildError.Error(), "曝險金額")
	assert.NotContains(t, buildError.Error(), "調低槓桿倍數")
}

// aBorrowedShortOnlyReplay is the same borrowed replay under rules that only ever
// short, entered the way those rules enter: on a sell.
func aBorrowedShortOnlyReplay(secondBar bar) leveragedReplaySpec {
	return leveragedReplaySpec{
		initialCapital: "10000", sizingMode: "allIn", tradingMode: "shortOnly",
		multiplier: "5", maintenanceMarginRate: "0.5",
		bars:    []bar{anEntryBar(), secondBar},
		signals: []vo.SignalVo{sellSignal, holdSignal},
	}
}

// Short-only borrows, and what it borrows can be called in. Its liquidation sits
// above the entry because up is the only way its position can go wrong — the mirror
// of every long the replay has been asked about until now.
//
// It is asserted through this mode rather than inferred from the long-short short:
// the side is read off the position's direction, and nothing had yet checked that
// a mode which can only produce shorts arrives there.
func TestLeveragedReplayWipesOutAShortOnlyPositionOnTheWayUp(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedShortOnlyReplay(bar{high: 120, low: 100, close: 115}))

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.PositionDirectionShort), result.ClosedTrades[0].Direction)
	assert.Equal(t, string(vo.TradeExitReasonLiquidation), result.ClosedTrades[0].ExitReason)
	// Above the entry of 100, not below it.
	assert.Equal(t, "119.5", result.ClosedTrades[0].ExitPrice.String())
	assert.Equal(t, "0", result.Summary.FinalEquity.String())
}

// Falling is where this position is right, so nothing calls the loan in.
func TestLeveragedReplayLeavesAShortOnlyPositionAloneOnTheWayDown(t *testing.T) {
	result := leveragedReplayOf(t, aBorrowedShortOnlyReplay(bar{high: 100, low: 80, close: 80}))

	assert.Empty(t, result.ClosedTrades)
	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
}

// The stop is nearer than the liquidation, and both are above the entry.
func TestLeveragedReplayTakesTheNearerExitAboveAShortOnlyEntry(t *testing.T) {
	spec := aBorrowedShortOnlyReplay(bar{high: 120, low: 100, close: 115})
	spec.stopLossPercentage = "5"

	result := leveragedReplayOf(t, spec)

	require.Len(t, result.ClosedTrades, 1)
	assert.Equal(t, string(vo.TradeExitReasonStopLoss), result.ClosedTrades[0].ExitReason)
	assert.Equal(t, "105", result.ClosedTrades[0].ExitPrice.String())
}

// Without a loan there is nobody to call one in, whichever way the position faces.
func TestLeveragedReplayNeverLiquidatesAnUnborrowedShortOnlyPosition(t *testing.T) {
	spec := aBorrowedShortOnlyReplay(bar{high: 200, low: 100, close: 200})
	spec.multiplier = ""

	result := leveragedReplayOf(t, spec)

	assert.Equal(t, 0, result.Summary.LiquidationExitCount)
}

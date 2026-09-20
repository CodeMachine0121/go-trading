package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leverageOf builds the settings the way a replay does, under long-short rules unless
// a test says otherwise.
func leverageOf(
	t *testing.T, multiplier string, maintenanceMarginRate string,
) (domains.BacktestLeverageDomain, error) {
	t.Helper()

	return domains.NewBacktestLeverageDomain(
		decimal.RequireFromString(multiplier),
		decimal.RequireFromString(maintenanceMarginRate),
		tradingModeOf(t, "longShort"))
}

// The multiplier is the switch. Three spellings mean the same thing — nothing is
// borrowed — and the reason is not that one times rarely gets wiped out: a position
// paid for in full has no lender, so there is nobody who could call it in.
func TestBacktestLeverageTreatsNothingZeroAndOneAsBorrowingNothing(t *testing.T) {
	testCases := []struct {
		name       string
		multiplier string
	}{
		{name: "nothing declared", multiplier: "0"},
		{name: "one, written out", multiplier: "1"},
		{name: "one, written as a decimal", multiplier: "1.0"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leverage, buildError := leverageOf(t, testCase.multiplier, "0")
			require.NoError(t, buildError)

			assert.Equal(t, "1", leverage.Multiplier().String())
			assert.Equal(t, "7", leverage.ExposureFrom(decimal.NewFromInt(7)).String())

			_, canBeLiquidated := leverage.AdverseDistance()
			assert.False(t, canBeLiquidated)
		})
	}
}

// Whatever the rate says, it cannot matter to a replay that borrowed nothing — there
// is no loan for it to be a fraction of.
func TestBacktestLeverageIgnoresTheRateWhenNothingIsBorrowed(t *testing.T) {
	leverage, buildError := leverageOf(t, "0", "50")
	require.NoError(t, buildError)

	_, canBeLiquidated := leverage.AdverseDistance()
	assert.False(t, canBeLiquidated)
}

func TestBacktestLeverageMultipliesWhatAStakeExposes(t *testing.T) {
	leverage, buildError := leverageOf(t, "5", "0.5")
	require.NoError(t, buildError)

	assert.Equal(t, "50000", leverage.ExposureFrom(decimal.NewFromInt(10000)).String())
}

// How far a position may fall is the loan itself: put down a fifth and a fifth is all
// there is to lose, less whatever the venue keeps back.
func TestBacktestLeverageWorksOutHowFarAPositionMayFall(t *testing.T) {
	testCases := []struct {
		name                  string
		multiplier            string
		maintenanceMarginRate string
		expectedDistance      string
	}{
		{name: "two times", multiplier: "2", maintenanceMarginRate: "0.5", expectedDistance: "49.5"},
		{name: "five times", multiplier: "5", maintenanceMarginRate: "0.5", expectedDistance: "19.5"},
		{name: "twenty times", multiplier: "20", maintenanceMarginRate: "0.5", expectedDistance: "4.5"},
		{name: "the rate eats into it", multiplier: "5", maintenanceMarginRate: "19", expectedDistance: "1"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leverage, buildError := leverageOf(
				t, testCase.multiplier, testCase.maintenanceMarginRate)
			require.NoError(t, buildError)

			adverseDistance, canBeLiquidated := leverage.AdverseDistance()
			require.True(t, canBeLiquidated)
			assert.Equal(t, testCase.expectedDistance, adverseDistance.String())
		})
	}
}

// Saying nothing about the rate is not the same as saying nothing about the
// multiplier. Once money is borrowed somebody is watching the collateral, so a blank
// gets the figure the venues use rather than switching the whole thing off.
func TestBacktestLeverageFallsBackToTheRateVenuesUse(t *testing.T) {
	declared, declaredError := leverageOf(t, "5", "0.5")
	require.NoError(t, declaredError)

	blank, blankError := leverageOf(t, "5", "0")
	require.NoError(t, blankError)

	declaredDistance, _ := declared.AdverseDistance()
	blankDistance, _ := blank.AdverseDistance()
	assert.Equal(t, declaredDistance.String(), blankDistance.String())
	assert.Equal(t, "19.5", blankDistance.String())
}

func TestBacktestLeverageRefusesFiguresItCannotTrade(t *testing.T) {
	testCases := []struct {
		name                  string
		multiplier            string
		maintenanceMarginRate string
		tradingMode           string
		expectedMessage       string
	}{
		{
			name:                  "half a times was meant as half a position",
			multiplier:            "0.5",
			maintenanceMarginRate: "0",
			tradingMode:           "longShort",
			expectedMessage:       "槓桿倍數不得小於 1 倍",
		},
		{
			name:                  "a negative multiplier is the same mistake",
			multiplier:            "-2",
			maintenanceMarginRate: "0",
			tradingMode:           "longShort",
			expectedMessage:       "槓桿倍數不得小於 1 倍",
		},
		{
			name:                  "a negative rate means a wiped-out position still holds",
			multiplier:            "5",
			maintenanceMarginRate: "-1",
			tradingMode:           "longShort",
			expectedMessage:       "維持保證金率不得為負——負的維持保證金等於倉位賠光了還撐得住",
		},
		{
			name:                  "spot has nobody to borrow from",
			multiplier:            "3",
			maintenanceMarginRate: "0",
			tradingMode:           "spot",
			expectedMessage:       "現貨交易模式開不了槓桿——現貨是拿現金換東西，沒有人借錢給你",
		},
		{
			name:                  "a rate that leaves no room at all",
			multiplier:            "5",
			maintenanceMarginRate: "20",
			tradingMode:           "longShort",
			expectedMessage: "維持保證金率必須小於 20%——5 倍槓桿下，" +
				"押下去的錢只夠讓價格逆著走這麼多，再多這一注在開倉那一棒就已經撐不住",
		},
		{
			name:                  "a rate past the room there is",
			multiplier:            "5",
			maintenanceMarginRate: "25",
			tradingMode:           "longShort",
			expectedMessage: "維持保證金率必須小於 20%——5 倍槓桿下，" +
				"押下去的錢只夠讓價格逆著走這麼多，再多這一注在開倉那一棒就已經撐不住",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewBacktestLeverageDomain(
				decimal.RequireFromString(testCase.multiplier),
				decimal.RequireFromString(testCase.maintenanceMarginRate),
				tradingModeOf(t, testCase.tradingMode))

			require.Error(t, buildError)
			assert.Equal(t, testCase.expectedMessage, buildError.Error())
		})
	}
}

// Spot is refused only when something is actually being borrowed. A spot replay that
// says nothing is the one every existing caller sends.
func TestBacktestLeverageLetsSpotThroughWhenNothingIsBorrowed(t *testing.T) {
	for _, multiplier := range []string{"0", "1"} {
		leverage, buildError := domains.NewBacktestLeverageDomain(
			decimal.RequireFromString(multiplier), decimal.Zero, tradingModeOf(t, "spot"))

		require.NoError(t, buildError)
		assert.Equal(t, "1", leverage.Multiplier().String())
	}
}

// The zero value is a replay that borrows nothing, so that every model downstream can
// hold one without asking whether there is any.
func TestBacktestLeverageZeroValueBorrowsNothing(t *testing.T) {
	leverage := domains.BacktestLeverageDomain{}

	assert.Equal(t, "1", leverage.Multiplier().String())
	assert.Equal(t, "250", leverage.ExposureFrom(decimal.NewFromInt(250)).String())

	_, canBeLiquidated := leverage.AdverseDistance()
	assert.False(t, canBeLiquidated)
}

// Where the loan is called in, priced. A long's sits below its entry and a short's
// above, because it is the price moving against the position — the same side the stop
// is on, which is the whole reason only one of the two can ever be reached.
func TestBacktestExitLevelsPlacesTheLiquidationPriceAgainstThePosition(t *testing.T) {
	testCases := []struct {
		name          string
		direction     vo.PositionDirectionVo
		multiplier    string
		expectedPrice string
	}{
		{
			name: "a long at five times", direction: vo.PositionDirectionLong,
			multiplier: "5", expectedPrice: "80.5",
		},
		{
			name: "a short at five times", direction: vo.PositionDirectionShort,
			multiplier: "5", expectedPrice: "119.5",
		},
		{
			name: "a long at two times falls further first", direction: vo.PositionDirectionLong,
			multiplier: "2", expectedPrice: "50.5",
		},
		{
			name: "a long at twenty times has almost no room", direction: vo.PositionDirectionLong,
			multiplier: "20", expectedPrice: "95.5",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leverage, buildError := leverageOf(t, testCase.multiplier, "0.5")
			require.NoError(t, buildError)

			exitPrices := domains.BacktestExitLevelsDomain{}.PricesFrom(
				testCase.direction, decimal.NewFromInt(100), leverage)

			require.True(t, exitPrices.HasAdverse)
			assert.Equal(t, vo.TradeExitReasonLiquidation, exitPrices.AdverseReason)
			assert.Equal(t, testCase.expectedPrice, exitPrices.AdversePrice.String())
		})
	}
}

// Borrowing nothing leaves the exits exactly where they were, which is what every
// report card produced before this model existed depends on.
func TestBacktestExitLevelsHasNoLiquidationPriceWithoutALoan(t *testing.T) {
	exitPrices := domains.BacktestExitLevelsDomain{}.PricesFrom(
		vo.PositionDirectionLong, decimal.NewFromInt(100), domains.BacktestLeverageDomain{})

	assert.False(t, exitPrices.HasAdverse)
}

// The two same-side exits, resolved at entry. Which one survives is decided by
// distance, and a tie goes to the one the caller asked for.
func TestBacktestExitLevelsKeepsOnlyTheNearerAdverseExit(t *testing.T) {
	testCases := []struct {
		name               string
		stopLossPercentage string
		multiplier         string
		expectedPrice      string
		expectedReason     vo.TradeExitReasonVo
	}{
		{
			name:               "a stop inside the liquidation price takes over",
			stopLossPercentage: "5", multiplier: "5",
			expectedPrice: "95", expectedReason: vo.TradeExitReasonStopLoss,
		},
		{
			name:               "a stop beyond it never gets a turn",
			stopLossPercentage: "30", multiplier: "5",
			expectedPrice: "80.5", expectedReason: vo.TradeExitReasonLiquidation,
		},
		{
			name:               "no stop at all leaves the liquidation price",
			stopLossPercentage: "0", multiplier: "5",
			expectedPrice: "80.5", expectedReason: vo.TradeExitReasonLiquidation,
		},
		{
			name:               "the two landing on the same price goes to the stop",
			stopLossPercentage: "19.5", multiplier: "5",
			expectedPrice: "80.5", expectedReason: vo.TradeExitReasonStopLoss,
		},
		{
			name:               "a stop with no loan behind it is the only adverse exit",
			stopLossPercentage: "5", multiplier: "0",
			expectedPrice: "95", expectedReason: vo.TradeExitReasonStopLoss,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			exitLevels, exitLevelsError := domains.NewBacktestExitLevelsDomain(
				decimal.RequireFromString(testCase.stopLossPercentage), decimal.Zero)
			require.NoError(t, exitLevelsError)

			leverage, leverageError := leverageOf(t, testCase.multiplier, "0.5")
			require.NoError(t, leverageError)

			exitPrices := exitLevels.PricesFrom(
				vo.PositionDirectionLong, decimal.NewFromInt(100), leverage)

			require.True(t, exitPrices.HasAdverse)
			assert.Equal(t, testCase.expectedPrice, exitPrices.AdversePrice.String())
			assert.Equal(t, testCase.expectedReason, exitPrices.AdverseReason)
		})
	}
}

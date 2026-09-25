package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anEntryPrice is 100 so a percentage distance and a price offset are the same number.
func anEntryPrice() decimal.Decimal {
	return decimal.NewFromInt(100)
}

func exitPricesUnderTest(
	t *testing.T, stopLoss int64, takeProfit int64,
) vo.ExitPricesVo {
	t.Helper()

	exitLevels, buildError := domains.NewBacktestExitLevelsDomain(
		decimal.NewFromInt(stopLoss), decimal.NewFromInt(takeProfit))
	require.NoError(t, buildError)

	return exitLevels.PricesFrom(anEntryPrice())
}

// A stop on the wrong side is still a plausible price, so only concrete figures catch it.
func TestBacktestExitLevelsPlacesTheTwoExitsOnOppositeSides(t *testing.T) {
	exitPrices := exitPricesUnderTest(t, 2, 5)

	assert.True(t, exitPrices.HasStopLoss)
	assert.Equal(t, "98", exitPrices.StopLossPrice.String())
	assert.True(t, exitPrices.HasTakeProfit)
	assert.Equal(t, "105", exitPrices.TakeProfitPrice.String())
}

// The zero value simulates no exits.
func TestBacktestExitLevelsZeroValueHasNoExitsAtAll(t *testing.T) {
	exitPrices := domains.BacktestExitLevelsDomain{}.PricesFrom(anEntryPrice())

	assert.False(t, exitPrices.HasStopLoss)
	assert.False(t, exitPrices.HasTakeProfit)
}

func TestBacktestExitLevelsTakesOneDistanceWithoutTheOther(t *testing.T) {
	stopOnly := exitPricesUnderTest(t, 2, 0)
	assert.True(t, stopOnly.HasStopLoss)
	assert.False(t, stopOnly.HasTakeProfit)

	targetOnly := exitPricesUnderTest(t, 0, 5)
	assert.False(t, targetOnly.HasStopLoss)
	assert.True(t, targetOnly.HasTakeProfit)
}

// A 100% stop prices at exactly zero, matching the bot's position plan.
func TestBacktestExitLevelsAllowsTheWholePriceAsADistance(t *testing.T) {
	exitPrices := exitPricesUnderTest(t, 100, 100)

	assert.True(t, exitPrices.HasStopLoss)
	assert.Equal(t, "0", exitPrices.StopLossPrice.String())
	assert.Equal(t, "200", exitPrices.TakeProfitPrice.String())
}

func TestBacktestExitLevelsRefusesDistancesThatCannotBePlaced(t *testing.T) {
	testCases := []struct {
		name            string
		stopLoss        string
		takeProfit      string
		expectedMessage string
	}{
		{
			name:     "a negative stop would sit on the other side of the price",
			stopLoss: "-2", takeProfit: "5",
			expectedMessage: "停損距離不得為負",
		},
		{
			name:     "a stop past the whole price would be priced below zero",
			stopLoss: "120", takeProfit: "5",
			expectedMessage: "停損距離不得超過 100%",
		},
		{
			name:     "and the target is checked by the very same rules",
			stopLoss: "2", takeProfit: "-5",
			expectedMessage: "停利距離不得為負",
		},
		{
			name:     "including its upper bound",
			stopLoss: "2", takeProfit: "120",
			expectedMessage: "停利距離不得超過 100%",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewBacktestExitLevelsDomain(
				decimal.RequireFromString(testCase.stopLoss),
				decimal.RequireFromString(testCase.takeProfit))

			require.Error(t, buildError)
			assert.Contains(t, buildError.Error(), testCase.expectedMessage)
		})
	}
}

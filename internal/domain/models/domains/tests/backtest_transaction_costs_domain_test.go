package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transactionCostsOf builds the model from two rates written the way a caller types
// them, and fails the test if they were refused. Cases that expect a refusal build it
// themselves.
func transactionCostsOf(
	t *testing.T, entryCostPercentage string, exitCostPercentage string,
) domains.BacktestTransactionCostsDomain {
	t.Helper()

	transactionCosts, buildError := domains.NewBacktestTransactionCostsDomain(
		decimalOf(t, entryCostPercentage), decimalOf(t, exitCostPercentage))
	require.NoError(t, buildError)

	return transactionCosts
}

func decimalOf(t *testing.T, value string) decimal.Decimal {
	t.Helper()

	parsedValue, parseError := decimal.NewFromString(value)
	require.NoError(t, parseError)

	return parsedValue
}

func TestBacktestTransactionCostsRefuseRatesThatCannotBeCharged(t *testing.T) {
	testCases := []struct {
		name                string
		entryCostPercentage string
		exitCostPercentage  string
		expectedReason      string
	}{
		{
			name:                "a negative entry rate would pay the trader to trade",
			entryCostPercentage: "-1",
			exitCostPercentage:  "0",
			expectedReason:      "進場成本率不得為負——負的成本等於交易就送錢",
		},
		{
			name:                "a negative exit rate is refused in its own words",
			entryCostPercentage: "0",
			exitCostPercentage:  "-0.5",
			expectedReason:      "出場成本率不得為負——負的成本等於交易就送錢",
		},
		{
			name:                "an entry rate past a hundred charges more than changed hands",
			entryCostPercentage: "101",
			exitCostPercentage:  "0",
			expectedReason:      "進場成本率不得超過 100%——成本不會超過成交金額本身",
		},
		{
			name:                "an exit rate past a hundred is refused the same way",
			entryCostPercentage: "0",
			exitCostPercentage:  "101",
			expectedReason:      "出場成本率不得超過 100%——成本不會超過成交金額本身",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewBacktestTransactionCostsDomain(
				decimalOf(t, testCase.entryCostPercentage),
				decimalOf(t, testCase.exitCostPercentage))

			require.Error(t, buildError)
			assert.Equal(t, testCase.expectedReason, buildError.Error())
		})
	}
}

// A rate of exactly a hundred is absurd and still arithmetic: the whole of what
// changed hands goes to the charge. Refusing it would need a rule that says where
// absurd begins, and there is no such place.
func TestBacktestTransactionCostsAllowExactlyAHundred(t *testing.T) {
	transactionCosts := transactionCostsOf(t, "100", "100")

	assert.Equal(t, "5050",
		transactionCosts.MaximumStakeFrom(decimal.NewFromInt(10100)).String())
	assert.Equal(t, "5050",
		transactionCosts.EntryCostFor(decimal.NewFromInt(5050)).String())
}

// The rate left out is the entry rate charged again. These two boxes are the halves of
// one thing — what trading costs — which is why one of them can stand in for the other,
// and why the two exit distances beside them deliberately cannot.
func TestBacktestTransactionCostsFallBackToTheEntryRateOnTheWayOut(t *testing.T) {
	testCases := []struct {
		name                string
		entryCostPercentage string
		exitCostPercentage  string
		expectedEntryCost   string
		expectedExitCost    string
	}{
		{
			name:                "naming only the entry rate charges it at both ends",
			entryCostPercentage: "1",
			exitCostPercentage:  "0",
			expectedEntryCost:   "100",
			expectedExitCost:    "100",
		},
		{
			name:                "naming both keeps them apart",
			entryCostPercentage: "0.0855",
			exitCostPercentage:  "0.3855",
			expectedEntryCost:   "8.55",
			expectedExitCost:    "38.55",
		},
		{
			name:                "naming only the exit rate leaves the entry free",
			entryCostPercentage: "0",
			exitCostPercentage:  "1",
			expectedEntryCost:   "0",
			expectedExitCost:    "100",
		},
		{
			name:                "naming neither charges nothing at all",
			entryCostPercentage: "0",
			exitCostPercentage:  "0",
			expectedEntryCost:   "0",
			expectedExitCost:    "0",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			transactionCosts := transactionCostsOf(
				t, testCase.entryCostPercentage, testCase.exitCostPercentage)

			assert.Equal(t, testCase.expectedEntryCost,
				transactionCosts.EntryCostFor(decimal.NewFromInt(10000)).String())
			assert.Equal(t, testCase.expectedExitCost,
				transactionCosts.ExitCostFor(decimal.NewFromInt(10000)).String())
		})
	}
}

// The ceiling is what lets all three sizing modes share one answer to "can this
// opening happen": a stake at or below it can always pay its own entry charge.
func TestBacktestTransactionCostsMaximumStakeLeavesRoomForTheEntryCharge(t *testing.T) {
	testCases := []struct {
		name                 string
		entryCostPercentage  string
		availableCash        string
		expectedMaximumStake string
	}{
		{
			name:                 "no entry charge hands the cash straight back",
			entryCostPercentage:  "0",
			availableCash:        "10100",
			expectedMaximumStake: "10100",
		},
		{
			name:                 "a one percent charge leaves exactly its own room",
			entryCostPercentage:  "1",
			availableCash:        "10100",
			expectedMaximumStake: "10000",
		},
		{
			name:                 "a charge of a hundred percent splits the cash in two",
			entryCostPercentage:  "100",
			availableCash:        "10100",
			expectedMaximumStake: "5050",
		},
		{
			name:                 "nothing on hand can stake nothing",
			entryCostPercentage:  "1",
			availableCash:        "0",
			expectedMaximumStake: "0",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			transactionCosts := transactionCostsOf(t, testCase.entryCostPercentage, "0")

			assert.Equal(t, testCase.expectedMaximumStake,
				transactionCosts.MaximumStakeFrom(
					decimalOf(t, testCase.availableCash)).String())
		})
	}
}

// A rate that never divides evenly is the ordinary case — Taiwan's discounted
// commission is one — and the ceiling has to stay affordable anyway. Rounding to
// nearest could hand back a stake whose own charge no longer fits and leave the
// account a fraction below zero.
func TestBacktestTransactionCostsMaximumStakeStaysAffordableWhenTheDivisionNeverEnds(t *testing.T) {
	transactionCosts := transactionCostsOf(t, "0.0855", "0")
	availableCash := decimal.NewFromInt(1000000)

	maximumStake := transactionCosts.MaximumStakeFrom(availableCash)

	assert.False(t,
		maximumStake.Add(transactionCosts.EntryCostFor(maximumStake)).
			GreaterThan(availableCash),
		"staking %s and paying %s exceeds the %s on hand",
		maximumStake, transactionCosts.EntryCostFor(maximumStake), availableCash)
}

// Money leaving is money leaving. A price arriving from outside as a negative would
// otherwise turn the charge into income — a mistake that improves every report card
// it touches and never fails.
func TestBacktestTransactionCostsAreAlwaysACharge(t *testing.T) {
	transactionCosts := transactionCostsOf(t, "1", "1")

	assert.Equal(t, "100",
		transactionCosts.ExitCostFor(decimal.NewFromInt(-10000)).String())
}

package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// transactionCostsOf fails the test on refusal; cases expecting a refusal build the model themselves.
func transactionCostsOf(
	t *testing.T, entryCostPercentage string, exitCostPercentage string,
) domains.BacktestTransactionCostsDomain {
	t.Helper()

	transactionCosts, buildError := domains.NewBacktestTransactionCostsDomain(
		decimal.RequireFromString(entryCostPercentage),
		decimal.RequireFromString(exitCostPercentage))
	require.NoError(t, buildError)

	return transactionCosts
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
				decimal.RequireFromString(testCase.entryCostPercentage),
				decimal.RequireFromString(testCase.exitCostPercentage))

			require.Error(t, buildError)
			assert.Equal(t, testCase.expectedReason, buildError.Error())
		})
	}
}

// A 100% rate is absurd but still arithmetic, and there is no principled cutoff below it.
func TestBacktestTransactionCostsAllowExactlyAHundred(t *testing.T) {
	transactionCosts := transactionCostsOf(t, "100", "100")

	assert.Equal(t, "5050",
		transactionCosts.MaximumStakeFrom(decimal.NewFromInt(10100)).String())
	assert.Equal(t, "5050",
		transactionCosts.EntryCostFor(decimal.NewFromInt(5050)).String())
}

// An omitted exit rate falls back to the entry rate (unlike the two exit distances).
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

// A stake at or below the ceiling can always pay its own entry charge, which lets every sizing mode share one affordability check.
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
					decimal.RequireFromString(testCase.availableCash),
				).String())
		})
	}
}

// Swept rather than sampled: rounding to nearest could overdraw by a fraction for particular rate/amount pairs when the division never ends.
func TestBacktestTransactionCostsMaximumStakeStaysAffordableWhenTheDivisionNeverEnds(t *testing.T) {
	for _, entryCostPercentage := range []string{
		"0.0855", "0.1425", "0.3855", "1", "3", "7", "33.3333", "99.9999", "100",
	} {
		for _, availableCash := range []string{
			"0.01", "1", "7", "10000", "12345.6789", "1000000", "999999999.99",
		} {
			t.Run(entryCostPercentage+"% of "+availableCash, func(t *testing.T) {
				transactionCosts := transactionCostsOf(t, entryCostPercentage, "0")
				cash := decimal.RequireFromString(availableCash)

				maximumStake := transactionCosts.MaximumStakeFrom(cash)
				spent := maximumStake.Add(transactionCosts.EntryCostFor(maximumStake))

				assert.False(t, spent.GreaterThan(cash),
					"staking %s and paying its charge spends %s out of %s",
					maximumStake, spent, cash)
				assert.False(t, maximumStake.IsNegative())
			})
		}
	}
}

// A percentage above the affordable share can never open anything, so it is refused up front like a zero percentage.
func TestPositionSizingKnowsWhenItCouldNeverStake(t *testing.T) {
	testCases := []struct {
		name                string
		declaredMode        string
		declaredValue       string
		entryCostPercentage string
		neverStakes         bool
	}{
		{
			name:                "staking the lot as a percentage, with anything charged",
			declaredMode:        "percentage",
			declaredValue:       "100",
			entryCostPercentage: "0.0855",
			neverStakes:         true,
		},
		{
			name:                "a percentage just under the lot is caught too",
			declaredMode:        "percentage",
			declaredValue:       "99.99",
			entryCostPercentage: "1",
			neverStakes:         true,
		},
		{
			name:                "a percentage right at the affordable share still stakes",
			declaredMode:        "percentage",
			declaredValue:       "99",
			entryCostPercentage: "1",
			neverStakes:         false,
		},
		{
			name:                "the same percentage is fine when trading is free",
			declaredMode:        "percentage",
			declaredValue:       "100",
			entryCostPercentage: "0",
			neverStakes:         false,
		},
		{
			name:                "staking everything shrinks to fit, so it always stakes",
			declaredMode:        "allIn",
			declaredValue:       "0",
			entryCostPercentage: "100",
			neverStakes:         false,
		},
		{
			name:                "a fixed amount depends on a balance that moves, so it is not judged here",
			declaredMode:        "fixedAmount",
			declaredValue:       "1000000",
			entryCostPercentage: "100",
			neverStakes:         false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionSizing, sizingError := domains.NewPositionSizingDomain(
				testCase.declaredMode, decimal.RequireFromString(testCase.declaredValue))
			require.NoError(t, sizingError)

			assert.Equal(t, testCase.neverStakes, positionSizing.NeverStakesUnder(transactionCostsOf(t, testCase.entryCostPercentage, "0")))
		})
	}
}

// A negative price must never turn a charge into income.
func TestBacktestTransactionCostsAreAlwaysACharge(t *testing.T) {
	transactionCosts := transactionCostsOf(t, "1", "1")

	assert.Equal(t, "100",
		transactionCosts.ExitCostFor(decimal.NewFromInt(-10000)).String())
}

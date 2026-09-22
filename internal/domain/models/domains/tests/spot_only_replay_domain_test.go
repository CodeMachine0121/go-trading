package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Saying nothing, saying spot, and borrowing nothing all describe the replay this
// system performs, so all three are free to say.
func TestSpotOnlyReplayDomainAcceptsWhatAlreadyMeansSpot(t *testing.T) {
	testCases := []struct {
		name                string
		declaredTradingMode string
		declaredLeverage    string
	}{
		{
			name:                "nothing declared at all",
			declaredTradingMode: "",
			declaredLeverage:    "0",
		},
		{
			name:                "saying spot out loud asks for what it gets anyway",
			declaredTradingMode: "spot",
			declaredLeverage:    "0",
		},
		{
			name:                "spelling is read without regard to case",
			declaredTradingMode: "SPOT",
			declaredLeverage:    "0",
		},
		{
			name:                "the spelling is read without its surrounding space",
			declaredTradingMode: "  spot  ",
			declaredLeverage:    "0",
		},
		{
			name:                "one times is a position paid for in full",
			declaredTradingMode: "",
			declaredLeverage:    "1",
		},
		{
			name:                "one written as a decimal is the same one",
			declaredTradingMode: "",
			declaredLeverage:    "1.0",
		},
		{
			name:                "spot and one times together say the same thing twice",
			declaredTradingMode: "spot",
			declaredLeverage:    "1",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			declaredLeverage, parseError := decimal.NewFromString(testCase.declaredLeverage)
			require.NoError(t, parseError)

			_, refusal := domains.NewSpotOnlyReplayDomain(
				testCase.declaredTradingMode, declaredLeverage)

			assert.NoError(t, refusal)
		})
	}
}

// Every set of rules other than spot is refused, and refused in one sentence. Which
// of the three somebody asked for makes no difference to what they have to do next,
// and three sentences would only suggest three different fixes.
func TestSpotOnlyReplayDomainRefusesEveryOtherSetOfRules(t *testing.T) {
	testCases := []struct {
		name                string
		declaredTradingMode string
	}{
		{
			name:                "the one that used to be the default",
			declaredTradingMode: "longShort",
		},
		{
			name:                "only long, but borrowing",
			declaredTradingMode: "leveragedLong",
		},
		{
			name:                "only short",
			declaredTradingMode: "shortOnly",
		},
		{
			name:                "a spelling nothing recognises",
			declaredTradingMode: "banana",
		},
	}

	firstRefusalSentence := ""
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, refusal := domains.NewSpotOnlyReplayDomain(
				testCase.declaredTradingMode, decimal.Zero)

			require.Error(t, refusal)
			assert.Contains(t, refusal.Error(), "只重演現貨")

			// One sentence for all four. A reader told a different thing each time
			// would go looking for a different fix each time, and there is only one.
			if firstRefusalSentence == "" {
				firstRefusalSentence = refusal.Error()
			}
			assert.Equal(t, firstRefusalSentence, refusal.Error())
		})
	}
}

// Borrowing is refused because there is nobody here to borrow from — and a figure
// below one is refused for the older reason, in the older words.
func TestSpotOnlyReplayDomainRefusesBorrowing(t *testing.T) {
	testCases := []struct {
		name             string
		declaredLeverage string
		expectedSentence string
	}{
		{
			name:             "twenty times asks for money nobody here lends",
			declaredLeverage: "20",
			expectedSentence: "沒有人借錢給你",
		},
		{
			name:             "just over one is still somebody else's money",
			declaredLeverage: "1.0001",
			expectedSentence: "沒有人借錢給你",
		},
		{
			name:             "half a position is not a loan, and is refused as itself",
			declaredLeverage: "0.5",
			expectedSentence: "不得小於 1 倍",
		},
		{
			name:             "the same mistake wearing a minus sign",
			declaredLeverage: "-1",
			expectedSentence: "不得小於 1 倍",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			declaredLeverage, parseError := decimal.NewFromString(testCase.declaredLeverage)
			require.NoError(t, parseError)

			_, refusal := domains.NewSpotOnlyReplayDomain("", declaredLeverage)

			require.Error(t, refusal)
			assert.Contains(t, refusal.Error(), testCase.expectedSentence)
		})
	}
}

// Both wrong at once is still refused. Which of the two it names does not matter —
// what matters is that the run does not happen.
func TestSpotOnlyReplayDomainRefusesWhenBothAreWrong(t *testing.T) {
	_, refusal := domains.NewSpotOnlyReplayDomain("longShort", decimal.NewFromInt(20))

	assert.Error(t, refusal)
}

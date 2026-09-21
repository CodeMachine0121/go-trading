package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The figure is read the same way wherever it is typed. Two places take it — what a
// bot suggests each round, and what one replay simulates — and they are different
// things, but 0.5 means the same thing in both.
func TestLeverageMultiplierDomainReadsTheFigure(t *testing.T) {
	testCases := []struct {
		name               string
		declaredMultiplier string
		expectsBorrowing   bool
		expectedMultiplier string
	}{
		{
			name:               "nothing typed at all is no loan",
			declaredMultiplier: "0",
			expectsBorrowing:   false,
			expectedMultiplier: "1",
		},
		{
			name:               "exactly one is a position paid for in full",
			declaredMultiplier: "1",
			expectsBorrowing:   false,
			expectedMultiplier: "1",
		},
		{
			name:               "one written as a decimal is the same one",
			declaredMultiplier: "1.0",
			expectsBorrowing:   false,
			expectedMultiplier: "1",
		},
		{
			name:               "above one is somebody else's money",
			declaredMultiplier: "1.8",
			expectsBorrowing:   true,
			expectedMultiplier: "1.8",
		},
		{
			name:               "and so is five times",
			declaredMultiplier: "5",
			expectsBorrowing:   true,
			expectedMultiplier: "5",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			multiplier, buildError := domains.NewLeverageMultiplierDomain(
				decimal.RequireFromString(testCase.declaredMultiplier))

			require.NoError(t, buildError)
			assert.Equal(t, testCase.expectsBorrowing, multiplier.IsBorrowed())
			// Borrowing nothing still multiplies by one, so no caller needs a branch.
			assert.Equal(t, testCase.expectedMultiplier, multiplier.Multiplier().String())
		})
	}
}

// Below one is refused rather than rounded up. Somebody who typed 0.5 meant half a
// position, and reading it as one would double what they asked for silently.
func TestLeverageMultiplierDomainRefusesBelowOne(t *testing.T) {
	for _, declaredMultiplier := range []string{"0.5", "0.99", "-2"} {
		t.Run(declaredMultiplier, func(t *testing.T) {
			_, buildError := domains.NewLeverageMultiplierDomain(
				decimal.RequireFromString(declaredMultiplier))

			require.Error(t, buildError)
			assert.Equal(t, "槓桿倍數不得小於 1 倍", buildError.Error())
		})
	}
}

func TestLeverageMultiplierDomainZeroValueBorrowsNothing(t *testing.T) {
	multiplier := domains.LeverageMultiplierDomain{}

	assert.False(t, multiplier.IsBorrowed())
	assert.Equal(t, "1", multiplier.Multiplier().String())
}

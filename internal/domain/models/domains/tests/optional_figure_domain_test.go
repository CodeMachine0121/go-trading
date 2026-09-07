package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func reportedFigure(value string) decimal.NullDecimal {
	return decimal.NewNullDecimal(decimal.RequireFromString(value))
}

// unreportedFigure is a figure this market does not publish at all. It is not zero:
// zero says the market traded none of it, this says the market never said.
func unreportedFigure() decimal.NullDecimal {
	return decimal.NullDecimal{}
}

func TestAnUnreportedFigureIsNotZero(t *testing.T) {
	unreported := domains.NewOptionalFigureDomain(unreportedFigure())
	tradedNone := domains.NewOptionalFigureDomain(reportedFigure("0"))

	assert.False(t, unreported.Value().Valid)
	assert.True(t, tradedNone.Value().Valid)
	assert.True(t, tradedNone.Value().Decimal.IsZero())
}

func TestPlusKeepsAnUnreportedFigureUnreported(t *testing.T) {
	testCases := []struct {
		name             string
		accumulated      decimal.NullDecimal
		added            decimal.NullDecimal
		expectedReported bool
		expectedValue    string
	}{
		{
			// Merging five-minute candles from a market that publishes no turnover must
			// produce an hourly candle that publishes no turnover either.
			name:        "nothing reported stays nothing reported",
			accumulated: unreportedFigure(), added: unreportedFigure(),
			expectedReported: false,
		},
		{
			name:        "two reported readings add up",
			accumulated: reportedFigure("100"), added: reportedFigure("23.5"),
			expectedReported: true, expectedValue: "123.5",
		},
		{
			// A market that reports the figure sometimes did trade that much. Throwing
			// the reading away because an earlier slot had none would understate it.
			name:        "a first reading arriving after nothing is that reading",
			accumulated: unreportedFigure(), added: reportedFigure("42"),
			expectedReported: true, expectedValue: "42",
		},
		{
			name:        "nothing added to a reading leaves it alone",
			accumulated: reportedFigure("42"), added: unreportedFigure(),
			expectedReported: true, expectedValue: "42",
		},
		{
			// Zero is a reading like any other, so it must survive as one.
			name:        "a reported zero is still reported",
			accumulated: unreportedFigure(), added: reportedFigure("0"),
			expectedReported: true, expectedValue: "0",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			summed := domains.NewOptionalFigureDomain(testCase.accumulated).Plus(testCase.added)

			assert.Equal(t, testCase.expectedReported, summed.Value().Valid)
			if testCase.expectedReported {
				assert.Equal(t, testCase.expectedValue, summed.Value().Decimal.String())
			}
		})
	}
}

func TestOnlyAReportedFigureCanBreakTheNoNegativesRule(t *testing.T) {
	testCases := []struct {
		name               string
		figure             decimal.NullDecimal
		expectedIsNegative bool
	}{
		{name: "a negative reading", figure: reportedFigure("-1"), expectedIsNegative: true},
		{name: "zero", figure: reportedFigure("0"), expectedIsNegative: false},
		{name: "a positive reading", figure: reportedFigure("12"), expectedIsNegative: false},
		// There is nothing to judge in a figure that was never reported.
		{name: "nothing reported", figure: unreportedFigure(), expectedIsNegative: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			isNegative := domains.NewOptionalFigureDomain(testCase.figure).IsNegative()

			assert.Equal(t, testCase.expectedIsNegative, isNegative)
		})
	}
}

func TestAnUnreportedFigureReachesAScriptAsZero(t *testing.T) {
	// A script is handed plain numbers and has no way to say "not reported". This is
	// the one place the distinction is knowingly given up, so it is pinned here rather
	// than discovered later.
	assert.InDelta(t, 0.0,
		domains.NewOptionalFigureDomain(unreportedFigure()).AsScriptFigure(), 0)
	assert.InDelta(t, 12.5,
		domains.NewOptionalFigureDomain(reportedFigure("12.5")).AsScriptFigure(), 0)
}

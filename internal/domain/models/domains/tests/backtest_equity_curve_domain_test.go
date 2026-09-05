package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordedCurveOf lays a curve down point by point, one hour apart.
func recordedCurveOf(initialCapital int64, equities ...int64) *domains.BacktestEquityCurveDomain {
	equityCurve := domains.NewBacktestEquityCurveDomain(decimal.NewFromInt(initialCapital))
	for pointIndex, equity := range equities {
		equityCurve.Record(
			replayStart.Add(time.Duration(pointIndex)*time.Hour), decimal.NewFromInt(equity))
	}

	return equityCurve
}

func TestBacktestEquityCurveDomainMaximumDrawdown(t *testing.T) {
	testCases := []struct {
		name             string
		initialCapital   int64
		equities         []int64
		expectedDrawdown float64
	}{
		{
			name:             "the worst fall from a peak, not the last one",
			initialCapital:   10000,
			equities:         []int64{10000, 12000, 9000, 11000},
			expectedDrawdown: 0.25,
		},
		{
			name:             "a curve that only rises never falls",
			initialCapital:   10000,
			equities:         []int64{10000, 11000, 12000},
			expectedDrawdown: 0,
		},
		{
			name:           "the starting capital is the first peak",
			initialCapital: 10000,
			// Every point is already below where the money started, so a curve that
			// took its peak from its own first point would report no fall at all.
			equities:         []int64{9500, 9000, 9200},
			expectedDrawdown: 0.10,
		},
		{
			name:             "a curve with no points has no fall",
			initialCapital:   10000,
			equities:         nil,
			expectedDrawdown: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			equityCurve := recordedCurveOf(testCase.initialCapital, testCase.equities...)

			assert.InDelta(t, testCase.expectedDrawdown, equityCurve.MaximumDrawdown(), 1e-9)
		})
	}
}

func TestBacktestEquityCurveDomainReadings(t *testing.T) {
	t.Run("the curve ends where its last point put it", func(t *testing.T) {
		equityCurve := recordedCurveOf(10000, 11000, 12500)

		assert.True(t, decimal.NewFromInt(12500).Equal(equityCurve.FinalEquity()))
		assert.InDelta(t, 0.25, equityCurve.TotalReturnRate(), 1e-9)
	})

	t.Run("a curve with no points ended where it started", func(t *testing.T) {
		equityCurve := recordedCurveOf(10000)

		assert.True(t, decimal.NewFromInt(10000).Equal(equityCurve.FinalEquity()))
		assert.InDelta(t, 0.0, equityCurve.TotalReturnRate(), 1e-9)
	})

	t.Run("a loss is reported as a negative return", func(t *testing.T) {
		equityCurve := recordedCurveOf(10000, 9200)

		assert.InDelta(t, -0.08, equityCurve.TotalReturnRate(), 1e-9)
	})

	t.Run("every recorded point is kept, in the order it was recorded", func(t *testing.T) {
		equityCurve := recordedCurveOf(10000, 10000, 11000, 9000)

		points := equityCurve.Points()
		require.Len(t, points, 3)
		assert.Equal(t, replayStart, points[0].OpenTime)
		assert.True(t, decimal.NewFromInt(11000).Equal(points[1].Equity))
		assert.Equal(t, replayStart.Add(2*time.Hour), points[2].OpenTime)
	})

	t.Run("an account wiped out has no peak left to fall from", func(t *testing.T) {
		// Starting from nothing there is no fall to measure, and nothing is divided by
		// zero to find that out.
		equityCurve := recordedCurveOf(0, 0, 0)

		assert.InDelta(t, 0.0, equityCurve.MaximumDrawdown(), 1e-9)
		assert.InDelta(t, 0.0, equityCurve.TotalReturnRate(), 1e-9)
	})
}

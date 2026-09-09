package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// windowNow is the moment every window below is settled against, so that "up to
// when" is decided by the arguments rather than by whenever the suite runs.
var windowNow = time.Date(2026, 9, 9, 10, 0, 0, 0, time.UTC)

func TestObservationWindowSettlesWhereTheStretchEnds(t *testing.T) {
	testCases := []struct {
		name            string
		declaredStart   time.Time
		declaredEnd     time.Time
		expectedEndTime time.Time
	}{
		{
			name:            "a moment already past is taken as it is",
			declaredStart:   windowNow.Add(-2 * time.Hour),
			declaredEnd:     windowNow.Add(-time.Hour),
			expectedEndTime: windowNow.Add(-time.Hour),
		},
		{
			name:            "no end named means now",
			declaredStart:   windowNow.Add(-2 * time.Hour),
			declaredEnd:     time.Time{},
			expectedEndTime: windowNow,
		},
		{
			name:            "an end that has not arrived means now",
			declaredStart:   windowNow.Add(-2 * time.Hour),
			declaredEnd:     windowNow.Add(time.Hour),
			expectedEndTime: windowNow,
		},
		{
			name:            "the boundary: an end exactly now is that moment",
			declaredStart:   windowNow.Add(-2 * time.Hour),
			declaredEnd:     windowNow,
			expectedEndTime: windowNow,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			observationWindow, buildError := domains.NewObservationWindowDomain(
				testCase.declaredStart, testCase.declaredEnd, windowNow)

			require.NoError(t, buildError)
			assert.Equal(t, testCase.expectedEndTime, observationWindow.EndTime())
			assert.Equal(t, testCase.declaredStart.UTC(), observationWindow.StartTime())
		})
	}
}

func TestObservationWindowRefusesAStretchNobodyCouldLookAt(t *testing.T) {
	testCases := []struct {
		name           string
		declaredStart  time.Time
		declaredEnd    time.Time
		expectedReason string
	}{
		{
			name:           "no beginning named",
			declaredStart:  time.Time{},
			declaredEnd:    windowNow,
			expectedReason: "必須指定要看哪一段的起點",
		},
		{
			name:           "it ends before it begins",
			declaredStart:  windowNow.Add(-time.Hour),
			declaredEnd:    windowNow.Add(-2 * time.Hour),
			expectedReason: "起點必須早於終點",
		},
		{
			name:           "the boundary: it begins exactly where it ends",
			declaredStart:  windowNow.Add(-time.Hour),
			declaredEnd:    windowNow.Add(-time.Hour),
			expectedReason: "起點必須早於終點",
		},
		{
			name:           "a beginning later than now, which the end is pulled back to",
			declaredStart:  windowNow.Add(time.Hour),
			declaredEnd:    time.Time{},
			expectedReason: "起點必須早於終點",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewObservationWindowDomain(
				testCase.declaredStart, testCase.declaredEnd, windowNow)

			require.ErrorIs(t, buildError, domains.ErrObservationWindowValidation)
			assert.Contains(t, buildError.Error(), testCase.expectedReason)
		})
	}
}

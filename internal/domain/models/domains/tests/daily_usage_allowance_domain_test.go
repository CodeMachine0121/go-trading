package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
)

func TestDailyUsageAllowanceIsSpentOnceItIsReached(t *testing.T) {
	testCases := []struct {
		name              string
		allowance         int
		usageToday        int
		expectedExhausted bool
	}{
		{name: "nothing used yet", allowance: 300000, usageToday: 0, expectedExhausted: false},
		{name: "part way through the day", allowance: 300000, usageToday: 150000, expectedExhausted: false},
		{name: "one short of the ceiling", allowance: 300000, usageToday: 299999, expectedExhausted: false},
		{
			// The allowance is what may be used, not what may be exceeded.
			name: "reaching the ceiling exactly spends it", allowance: 300000, usageToday: 300000,
			expectedExhausted: true,
		},
		{name: "past the ceiling is spent too", allowance: 300000, usageToday: 400000, expectedExhausted: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			allowance := domains.NewDailyUsageAllowanceDomain(
				testCase.allowance, time.Date(2026, 9, 4, 13, 45, 10, 0, time.UTC))

			assert.Equal(t, testCase.expectedExhausted, allowance.Exhausted(testCase.usageToday))
			assert.Equal(t, testCase.allowance, allowance.Allowance())
		})
	}
}

func TestDailyUsageAllowanceCutsTheDayAtUniversalMidnight(t *testing.T) {
	testCases := []struct {
		name               string
		now                time.Time
		expectedStartOfDay time.Time
		expectedResetsAt   time.Time
	}{
		{
			name:               "mid-afternoon",
			now:                time.Date(2026, 9, 4, 13, 45, 10, 0, time.UTC),
			expectedStartOfDay: time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC),
			expectedResetsAt:   time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			// Midnight itself belongs to the day it opens, not the one it closed.
			name:               "midnight itself",
			now:                time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC),
			expectedStartOfDay: time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC),
			expectedResetsAt:   time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:               "the last second of the day",
			now:                time.Date(2026, 9, 4, 23, 59, 59, 0, time.UTC),
			expectedStartOfDay: time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC),
			expectedResetsAt:   time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			// The same instant written in another zone is the same instant, so the
			// day it falls in is the universal one either way.
			name: "the same instant told in another zone",
			now: time.Date(2026, 9, 4, 21, 45, 10, 0,
				time.FixedZone("UTC+8", 8*60*60)),
			expectedStartOfDay: time.Date(2026, 9, 4, 0, 0, 0, 0, time.UTC),
			expectedResetsAt:   time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC),
		},
		{
			name:               "the last day of a month rolls into the next",
			now:                time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
			expectedStartOfDay: time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC),
			expectedResetsAt:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			allowance := domains.NewDailyUsageAllowanceDomain(300000, testCase.now)

			assert.Equal(t, testCase.expectedStartOfDay, allowance.StartOfDay())
			assert.Equal(t, testCase.expectedResetsAt, allowance.ResetsAt())
		})
	}
}

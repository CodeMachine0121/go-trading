package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractPositionStatisticHistoryDomainWalksFromTheDayTheLookbackReachesThroughToday(t *testing.T) {
	testCases := []struct {
		name             string
		lookbackDays     int
		expectedDayCount int
		expectedFirstDay time.Time
		expectedFinalDay time.Time
	}{
		{name: "回溯 180 天", lookbackDays: 180, expectedDayCount: 181,
			expectedFirstDay: time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC),
			expectedFinalDay: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)},
		{name: "回溯 1 天是昨天與今天", lookbackDays: 1, expectedDayCount: 2,
			expectedFirstDay: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC),
			expectedFinalDay: time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			historyDomain := domains.NewContractPositionStatisticHistoryDomain(
				time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
				time.Duration(testCase.lookbackDays)*24*time.Hour)

			days := historyDomain.Days()

			require.Len(t, days, testCase.expectedDayCount)
			assert.Equal(t, testCase.expectedFirstDay, days[0].Day)
			assert.Equal(t, testCase.expectedFinalDay, days[len(days)-1].Day)
		})
	}
}

func TestContractPositionStatisticHistoryDomainCoversADayFromMidnightToItsLastFiveMinutes(t *testing.T) {
	historyDomain := domains.NewContractPositionStatisticHistoryDomain(
		time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC), 24*time.Hour)

	firstDay := historyDomain.Days()[0]

	assert.Equal(t, time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), firstDay.FirstStatisticTime)
	assert.Equal(t, time.Date(2026, 9, 24, 23, 55, 0, 0, time.UTC), firstDay.LastStatisticTime)
}

func TestContractPositionStatisticHistoryDomainCallsADayWholeAtOneStatisticEveryFiveMinutes(t *testing.T) {
	historyDomain := domains.NewContractPositionStatisticHistoryDomain(
		time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC), 24*time.Hour)

	testCases := []struct {
		heldCount int
		expected  bool
	}{
		{heldCount: 288, expected: true},
		{heldCount: 287, expected: false},
		{heldCount: 289, expected: true},
		{heldCount: 0, expected: false},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expected, historyDomain.IsDayComplete(testCase.heldCount),
			"已存 %d 筆", testCase.heldCount)
	}
}

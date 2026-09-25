package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKCandleHistorySyncCapacityAdmitsWhileAPlaceIsFree(t *testing.T) {
	testCases := []struct {
		name                   string
		maximumConcurrentSyncs int
		runningCount           int
	}{
		{name: "nothing running", maximumConcurrentSyncs: 2, runningCount: 0},
		{name: "the last place", maximumConcurrentSyncs: 2, runningCount: 1},
		{name: "a limit of one with nothing running", maximumConcurrentSyncs: 1, runningCount: 0},
		{name: "a limit of zero still holds one", maximumConcurrentSyncs: 0, runningCount: 0},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			capacity := domains.NewKCandleHistorySyncCapacityDomain(testCase.maximumConcurrentSyncs)

			assert.NoError(t, capacity.Admit(testCase.runningCount))
		})
	}
}

func TestKCandleHistorySyncCapacityRefusesOnceEveryPlaceIsTaken(t *testing.T) {
	testCases := []struct {
		name                   string
		maximumConcurrentSyncs int
		runningCount           int
		expectedMessage        string
	}{
		{name: "exactly full", maximumConcurrentSyncs: 2, runningCount: 2,
			expectedMessage: "同時最多 2 趟歷史同步，等其中一趟結束再開"},
		{name: "above a limit lowered since", maximumConcurrentSyncs: 2, runningCount: 3,
			expectedMessage: "同時最多 2 趟"},
		{name: "a limit of one", maximumConcurrentSyncs: 1, runningCount: 1,
			expectedMessage: "同時最多 1 趟"},
		{name: "a negative limit is one", maximumConcurrentSyncs: -1, runningCount: 1,
			expectedMessage: "同時最多 1 趟"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			capacity := domains.NewKCandleHistorySyncCapacityDomain(testCase.maximumConcurrentSyncs)

			admitError := capacity.Admit(testCase.runningCount)

			require.ErrorIs(t, admitError, domains.ErrKCandleHistorySyncCapacityReached)
			assert.Contains(t, admitError.Error(), testCase.expectedMessage)
		})
	}
}

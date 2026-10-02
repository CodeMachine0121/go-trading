package persistence_test

import (
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testLeaseName = "background-jobs"

var leaseTime = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)

func atSeconds(seconds int) time.Time {
	return leaseTime.Add(time.Duration(seconds) * time.Second)
}

func TestJobLeadershipLeaseRepositoryAcquire(t *testing.T) {
	testCases := []struct {
		name          string
		holderFirst   string
		holderSecond  string
		secondAt      time.Time
		secondExpires time.Time
		expected      bool
	}{
		{
			name: "another replica cannot take a lease that has not expired", holderFirst: "replica-a",
			holderSecond: "replica-b", secondAt: atSeconds(10), secondExpires: atSeconds(40), expected: false,
		},
		{
			name: "the holder extends its own lease", holderFirst: "replica-a",
			holderSecond: "replica-a", secondAt: atSeconds(10), secondExpires: atSeconds(40), expected: true,
		},
		{
			name: "another replica takes an expired lease", holderFirst: "replica-a",
			holderSecond: "replica-b", secondAt: atSeconds(31), secondExpires: atSeconds(61), expected: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := persistence.NewJobLeadershipLeaseRepository(newTestDatabase(t))

			firstAcquired, firstError := repository.Acquire(
				t.Context(), testLeaseName, testCase.holderFirst, leaseTime, atSeconds(30))
			require.NoError(t, firstError)
			require.True(t, firstAcquired, "with no lease stored, the first replica becomes the holder")

			secondAcquired, secondError := repository.Acquire(
				t.Context(), testLeaseName, testCase.holderSecond, testCase.secondAt, testCase.secondExpires)
			require.NoError(t, secondError)

			assert.Equal(t, testCase.expected, secondAcquired)
		})
	}
}

func TestJobLeadershipLeaseRepositoryARenewedLeaseKeepsOthersOutPastTheOldExpiry(t *testing.T) {
	repository := persistence.NewJobLeadershipLeaseRepository(newTestDatabase(t))
	_, _ = repository.Acquire(t.Context(), testLeaseName, "replica-a", leaseTime, atSeconds(30))
	_, _ = repository.Acquire(t.Context(), testLeaseName, "replica-a", atSeconds(10), atSeconds(40))

	acquired, acquireError := repository.Acquire(
		t.Context(), testLeaseName, "replica-b", atSeconds(35), atSeconds(65))
	require.NoError(t, acquireError)

	assert.False(t, acquired)
}

func TestJobLeadershipLeaseRepositoryRelease(t *testing.T) {
	testCases := []struct {
		name        string
		releasedBy  string
		expectTaken bool
	}{
		{name: "the holder's release lets the next replica in at once", releasedBy: "replica-a", expectTaken: true},
		{name: "a release by someone else leaves the holder in place", releasedBy: "replica-b", expectTaken: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			repository := persistence.NewJobLeadershipLeaseRepository(newTestDatabase(t))
			_, _ = repository.Acquire(t.Context(), testLeaseName, "replica-a", leaseTime, atSeconds(30))

			require.NoError(t, repository.Release(t.Context(), testLeaseName, testCase.releasedBy))
			acquired, acquireError := repository.Acquire(
				t.Context(), testLeaseName, "replica-c", atSeconds(1), atSeconds(31))
			require.NoError(t, acquireError)

			assert.Equal(t, testCase.expectTaken, acquired)
		})
	}
}

func TestJobLeadershipLeaseRepositoryExactlyOneOfManyRacingReplicasWins(t *testing.T) {
	repository := persistence.NewJobLeadershipLeaseRepository(newTestDatabase(t))
	replicaNames := []string{"replica-a", "replica-b", "replica-c", "replica-d", "replica-e"}
	winners := make([]bool, len(replicaNames))
	startTogether := sync.WaitGroup{}
	finished := sync.WaitGroup{}
	startTogether.Add(1)

	for index, replicaName := range replicaNames {
		finished.Add(1)
		go func() {
			defer finished.Done()
			startTogether.Wait()
			acquired, acquireError := repository.Acquire(
				t.Context(), testLeaseName, replicaName, leaseTime, atSeconds(30))
			winners[index] = acquireError == nil && acquired
		}()
	}
	startTogether.Done()
	finished.Wait()

	winnerCount := 0
	for _, won := range winners {
		if won {
			winnerCount++
		}
	}
	assert.Equal(t, 1, winnerCount)
}

package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const thisReplicaName = "go-trading-7d9f-abcde"

var (
	renewedAt           = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	renewedAtPlus24s    = time.Date(2026, 10, 2, 8, 0, 24, 0, time.UTC)
	renewedAtPlus25s    = time.Date(2026, 10, 2, 8, 0, 25, 0, time.UTC)
	renewedAtPlus26s    = time.Date(2026, 10, 2, 8, 0, 26, 0, time.UTC)
	leaseStoredUntil    = time.Date(2026, 10, 2, 8, 0, 30, 0, time.UTC)
	leaseStorageFailure = errors.New("storage unavailable")
)

type jobLeadershipApplicationUnderTest struct {
	jobLeadershipApplication     *application.JobLeadershipApplication
	jobLeadershipLeaseRepository *mocks.MockIJobLeadershipLeaseRepository
	clock                        *movableClock
}

// movableClock lets a test ask "is it still on duty" at a later moment without sleeping.
type movableClock struct {
	now time.Time
}

func newJobLeadershipApplicationUnderTest(t *testing.T) jobLeadershipApplicationUnderTest {
	mockController := gomock.NewController(t)
	jobLeadershipLeaseRepository := mocks.NewMockIJobLeadershipLeaseRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clock := &movableClock{now: renewedAt}
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time { return clock.now }).AnyTimes()

	return jobLeadershipApplicationUnderTest{
		jobLeadershipApplication: application.NewJobLeadershipApplication(service.NewJobLeadershipService(
			jobLeadershipLeaseRepository, clockProxy,
			domains.NewJobLeadershipTermDomain(30*time.Second, 5*time.Second), thisReplicaName)),
		jobLeadershipLeaseRepository: jobLeadershipLeaseRepository,
		clock:                        clock,
	}
}

func TestJobLeadershipApplicationTrustsTheDutyUntilJustBeforeTheLeaseEnds(t *testing.T) {
	testCases := []struct {
		name          string
		askedAt       time.Time
		expectedReply bool
	}{
		{name: "twenty-four seconds after renewing it is still on duty", askedAt: renewedAtPlus24s, expectedReply: true},
		{name: "at exactly twenty-five seconds it has already let go", askedAt: renewedAtPlus25s, expectedReply: false},
		{name: "twenty-six seconds after renewing it is no longer on duty", askedAt: renewedAtPlus26s, expectedReply: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newJobLeadershipApplicationUnderTest(t)
			underTest.jobLeadershipLeaseRepository.EXPECT().
				Acquire(gomock.Any(), gomock.Any(), thisReplicaName, renewedAt, leaseStoredUntil).
				Return(true, nil)

			_, renewError := underTest.jobLeadershipApplication.RenewLeadership(t.Context())
			require.NoError(t, renewError)
			underTest.clock.now = testCase.askedAt

			assert.Equal(t, testCase.expectedReply, underTest.jobLeadershipApplication.IsLeader())
		})
	}
}

func TestJobLeadershipApplicationRenewal(t *testing.T) {
	testCases := []struct {
		name           string
		previouslyHeld bool
		acquired       bool
		acquireError   error
		expectedLeader bool
		expectedChange dto.JobLeadershipChangeDto
	}{
		{
			name: "a replica that wins becomes the leader", previouslyHeld: false, acquired: true,
			expectedLeader: true, expectedChange: dto.JobLeadershipChangeDto{Gained: true},
		},
		{
			name: "a replica that loses to the holder is not the leader", previouslyHeld: false, acquired: false,
			expectedLeader: false, expectedChange: dto.JobLeadershipChangeDto{},
		},
		{
			name: "a leader still holding it reports no change", previouslyHeld: true, acquired: true,
			expectedLeader: true, expectedChange: dto.JobLeadershipChangeDto{},
		},
		{
			name: "a leader that is outbid reports the loss", previouslyHeld: true, acquired: false,
			expectedLeader: false, expectedChange: dto.JobLeadershipChangeDto{Lost: true},
		},
		{
			name: "a leader whose renewal fails keeps the duty until its last renewal runs out", previouslyHeld: true,
			acquireError: leaseStorageFailure, expectedLeader: true, expectedChange: dto.JobLeadershipChangeDto{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newJobLeadershipApplicationUnderTest(t)
			if testCase.previouslyHeld {
				underTest.jobLeadershipLeaseRepository.EXPECT().
					Acquire(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(true, nil)
				_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())
			}
			underTest.jobLeadershipLeaseRepository.EXPECT().
				Acquire(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(testCase.acquired, testCase.acquireError)

			change, renewError := underTest.jobLeadershipApplication.RenewLeadership(t.Context())

			assert.ErrorIs(t, renewError, testCase.acquireError)
			assert.Equal(t, testCase.expectedChange, change)
			assert.Equal(t, testCase.expectedLeader, underTest.jobLeadershipApplication.IsLeader())
		})
	}
}

func TestJobLeadershipApplicationReleaseGivesTheDutyBack(t *testing.T) {
	underTest := newJobLeadershipApplicationUnderTest(t)
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Release(gomock.Any(), gomock.Any(), thisReplicaName).Return(nil)

	require.NoError(t, underTest.jobLeadershipApplication.ReleaseLeadership(t.Context()))

	assert.False(t, underTest.jobLeadershipApplication.IsLeader())
}

func TestJobLeadershipApplicationReleaseDoesNothingWhenNeverOnDuty(t *testing.T) {
	underTest := newJobLeadershipApplicationUnderTest(t)
	// No Release expectation: the mock fails the test if it is called.

	require.NoError(t, underTest.jobLeadershipApplication.ReleaseLeadership(t.Context()))
}

func TestJobLeadershipApplicationAFailedRenewalAfterTheCutoffReportsTheLoss(t *testing.T) {
	underTest := newJobLeadershipApplicationUnderTest(t)
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), renewedAt, gomock.Any()).Return(true, nil)
	_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())
	underTest.clock.now = renewedAtPlus26s
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), renewedAtPlus26s, gomock.Any()).Return(false, leaseStorageFailure)

	change, renewError := underTest.jobLeadershipApplication.RenewLeadership(t.Context())

	assert.ErrorIs(t, renewError, leaseStorageFailure)
	assert.Equal(t, dto.JobLeadershipChangeDto{Lost: true}, change)
	assert.False(t, underTest.jobLeadershipApplication.IsLeader())
}

func TestJobLeadershipApplicationAFailedRenewalNeverExtendsTheDuty(t *testing.T) {
	underTest := newJobLeadershipApplicationUnderTest(t)
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), renewedAt, gomock.Any()).Return(true, nil)
	_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())
	underTest.clock.now = renewedAtPlus24s
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), renewedAtPlus24s, gomock.Any()).Return(false, leaseStorageFailure)
	_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())

	underTest.clock.now = renewedAtPlus26s

	assert.False(t, underTest.jobLeadershipApplication.IsLeader(),
		"the failed renewal at 24 seconds must not move the cutoff past 25")
}

func TestJobLeadershipApplicationWinningTheDutyBackAfterItRanOutIsAGain(t *testing.T) {
	underTest := newJobLeadershipApplicationUnderTest(t)
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), renewedAt, gomock.Any()).Return(true, nil)
	_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())
	underTest.clock.now = renewedAtPlus26s
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), renewedAtPlus26s, gomock.Any()).Return(false, leaseStorageFailure)
	_, _ = underTest.jobLeadershipApplication.RenewLeadership(t.Context())
	wonBackAt := renewedAtPlus26s.Add(10 * time.Second)
	underTest.clock.now = wonBackAt
	underTest.jobLeadershipLeaseRepository.EXPECT().
		Acquire(gomock.Any(), gomock.Any(), gomock.Any(), wonBackAt, gomock.Any()).Return(true, nil)

	change, renewError := underTest.jobLeadershipApplication.RenewLeadership(t.Context())

	require.NoError(t, renewError)
	assert.Equal(t, dto.JobLeadershipChangeDto{Gained: true}, change)
}

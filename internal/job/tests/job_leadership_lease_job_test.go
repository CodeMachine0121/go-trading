package job_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestJobLeadershipLeaseJobRenewsOnStartAndNeverReleasesOnStop(t *testing.T) {
	mockController := gomock.NewController(t)
	leaseRepository := mocks.NewMockIJobLeadershipLeaseRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	renewals := make(chan string, 8)
	leaseRepository.EXPECT().Acquire(gomock.Any(), gomock.Any(), "replica-a", gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, string, string, time.Time, time.Time) (bool, error) {
			renewals <- "renewed"
			return true, nil
		}).AnyTimes()
	// No Release expectation: stopping the job must leave giving the duty back to the server.
	leadership := application.NewJobLeadershipApplication(service.NewJobLeadershipService(
		leaseRepository, clockProxy, domains.NewJobLeadershipTermDomain(30*time.Second, 5*time.Second), "replica-a"))

	leaseJob := job.NewJobLeadershipLeaseJob(leadership, time.Hour)
	leaseJob.Start(t.Context())

	select {
	case <-renewals:
	case <-time.After(5 * time.Second):
		t.Fatal("the lease was not renewed on start")
	}
	leaseJob.Stop()

	assert.True(t, leadership.IsLeader())
}

func TestJobLeadershipLeaseJobSaysWhenTheDutyIsLostOrCannotBeRenewed(t *testing.T) {
	testCases := []struct {
		name          string
		acquireError  error
		expectedWords string
	}{
		{name: "another replica took the duty", expectedWords: "job leadership lost"},
		{name: "the duty could not be renewed", acquireError: errors.New("storage unavailable"),
			expectedWords: "could not be renewed"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorded := captureRecords(t)
			mockController := gomock.NewController(t)
			leaseRepository := mocks.NewMockIJobLeadershipLeaseRepository(mockController)
			clockProxy := mocks.NewMockIClockProxy(mockController)
			clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
			firstRenewal := leaseRepository.EXPECT().
				Acquire(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
			leaseRepository.EXPECT().Acquire(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
				Return(false, testCase.acquireError).After(firstRenewal).AnyTimes()
			leaseJob := job.NewJobLeadershipLeaseJob(application.NewJobLeadershipApplication(
				service.NewJobLeadershipService(leaseRepository, clockProxy,
					domains.NewJobLeadershipTermDomain(30*time.Second, 5*time.Second), "replica-a")),
				testInterval)
			t.Cleanup(leaseJob.Stop)

			leaseJob.Start(t.Context())

			recorded.waitFor(t, "job leadership gained")
			recorded.waitFor(t, testCase.expectedWords)
		})
	}
}

func TestPendingMessageDispatchJobSaysWhenTheQueueCannotBeRead(t *testing.T) {
	recorded := captureRecords(t)
	mockController := gomock.NewController(t)
	pendingMessageRepository := mocks.NewMockIPendingMessageRepository(mockController)
	pendingMessageRepository.EXPECT().DeleteSettledBefore(gomock.Any(), gomock.Any()).
		Return(errors.New("storage unavailable")).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	dispatchJob := job.NewPendingMessageDispatchJob(application.NewPendingMessageDispatchApplication(
		service.NewPendingMessageService(pendingMessageRepository, mocks.NewMockIStrategyBotRepository(mockController),
			mocks.NewMockITransactionRepository(mockController), nil, clockProxy, "replica-a", 2*time.Minute, 8)),
		time.Hour)
	t.Cleanup(dispatchJob.Stop)

	dispatchJob.Start(t.Context())

	assert.Contains(t, recorded.waitFor(t, "could not be sent this round"), "storage unavailable")
}

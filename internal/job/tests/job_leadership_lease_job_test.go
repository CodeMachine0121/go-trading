package job_test

import (
	"context"
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

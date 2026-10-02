package job_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// switchableDuty is a real leadership application over a mocked lease, so a test can put this replica on or off duty.
type switchableDuty struct {
	application *application.JobLeadershipApplication
	held        *atomic.Bool
	// clockReads ticks on every clock read, which every duty question makes, so a test can wait for real checks instead of sleeping.
	clockReads chan struct{}
}

func newDuty(t *testing.T, onDuty bool) switchableDuty {
	t.Helper()

	mockController := gomock.NewController(t)
	held := &atomic.Bool{}
	held.Store(onDuty)
	leaseRepository := mocks.NewMockIJobLeadershipLeaseRepository(mockController)
	leaseRepository.EXPECT().Acquire(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, string, string, time.Time, time.Time) (bool, error) {
			return held.Load(), nil
		}).AnyTimes()
	clockReads := make(chan struct{}, 1024)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		select {
		case clockReads <- struct{}{}:
		default:
		}

		return currentTime
	}).AnyTimes()

	duty := switchableDuty{
		clockReads: clockReads,
		application: application.NewJobLeadershipApplication(service.NewJobLeadershipService(
			leaseRepository, clockProxy, domains.NewJobLeadershipTermDomain(30*time.Second, 5*time.Second),
			"replica-under-test")),
		held: held,
	}
	duty.set(t, onDuty)

	return duty
}

func onDuty(t *testing.T) *application.JobLeadershipApplication {
	t.Helper()

	return newDuty(t, true).application
}

func (switchableDuty switchableDuty) set(t *testing.T, onDuty bool) {
	t.Helper()

	switchableDuty.held.Store(onDuty)
	_, renewError := switchableDuty.application.RenewLeadership(t.Context())
	require.NoError(t, renewError)
}

// waitForChecks waits until the job has asked about its duty count more times from now on.
func (switchableDuty switchableDuty) waitForChecks(t *testing.T, count int) {
	t.Helper()

	for {
		select {
		case <-switchableDuty.clockReads:
			continue
		default:
		}

		break
	}
	for range count {
		select {
		case <-switchableDuty.clockReads:
		case <-time.After(2 * time.Second):
			t.Fatal("the job stopped asking about its duty")
		}
	}
}

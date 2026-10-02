package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type interruptedWorkJobsUnderTest struct {
	application *application.InterruptedWorkApplication
	beats       chan string
	sweeps      chan string
}

func newInterruptedWorkJobsUnderTest(t *testing.T) interruptedWorkJobsUnderTest {
	t.Helper()

	controller := gomock.NewController(t)
	beats := make(chan string, 64)
	sweeps := make(chan string, 64)
	heartbeatRepository := mocks.NewMockIReplicaHeartbeatRepository(controller)
	heartbeatRepository.EXPECT().Beat(gomock.Any(), "replica-a", gomock.Any()).
		DoAndReturn(func(context.Context, string, time.Time) error {
			beats <- "beat"

			return nil
		}).AnyTimes()
	heartbeatRepository.EXPECT().FindSeenSince(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	conversationRepository := mocks.NewMockIConversationRepository(controller)
	conversationRepository.EXPECT().FailRunningTurnsOutside(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, []string, string) (int, error) {
			sweeps <- "sweep"

			return 1, nil
		}).AnyTimes()
	historySyncRunRepository := mocks.NewMockIKCandleHistorySyncRunRepository(controller)
	historySyncRunRepository.EXPECT().FailRunningOutside(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	contractHistorySyncRepository := mocks.NewMockIKCandleContractHistorySyncRunRepository(controller)
	contractHistorySyncRepository.EXPECT().FailRunningOutside(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(0, nil).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	catalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})

	return interruptedWorkJobsUnderTest{
		application: application.NewInterruptedWorkApplication(
			service.NewReplicaPresenceService(heartbeatRepository, clockProxy, "replica-a", 10*time.Second),
			service.NewAssistantConversationService(conversationRepository, mocks.NewMockIAssistantProxy(controller),
				[]domaininterface.IAssistantQuery{}, clockProxy, 20, 8, 300000, 2000, "replica-a"),
			service.NewKCandleIngestionService(mocks.NewMockIKCandleRepository(controller), historySyncRunRepository,
				mocks.NewMockITradingSymbolRepository(controller), mocks.NewMockIMarketDataProxy(controller), clockProxy,
				catalog, 5, 24*time.Hour, 2, "replica-a"),
			service.NewContractKCandleIngestionService(mocks.NewMockIKCandleContractRepository(controller),
				contractHistorySyncRepository, mocks.NewMockIContractTradingSymbolRepository(controller),
				mocks.NewMockIContractMarketDataProxy(controller), clockProxy, catalog, 5, 24*time.Hour, nil, 2, "replica-a")),
		beats:  beats,
		sweeps: sweeps,
	}
}

func TestReplicaHeartbeatJobBeatsOnStartAndThenEveryInterval(t *testing.T) {
	underTest := newInterruptedWorkJobsUnderTest(t)
	heartbeatJob := job.NewReplicaHeartbeatJob(underTest.application, testInterval)
	t.Cleanup(heartbeatJob.Stop)

	heartbeatJob.Start(t.Context())

	assert.Equal(t, "beat", nextFrom(t, underTest.beats))
	assert.Equal(t, "beat", nextFrom(t, underTest.beats))
}

func TestInterruptedWorkSweepJobSweepsOnlyOnDuty(t *testing.T) {
	underTest := newInterruptedWorkJobsUnderTest(t)
	duty := newDuty(t, false)
	sweepJob := job.NewInterruptedWorkSweepJob(underTest.application, duty.application, testInterval)
	t.Cleanup(sweepJob.Stop)
	sweepJob.Start(t.Context())

	duty.waitForChecks(t, 3)
	assert.Empty(t, underTest.sweeps, "off duty, nothing is swept")

	duty.set(t, true)
	assert.Equal(t, "sweep", nextFrom(t, underTest.sweeps))
}

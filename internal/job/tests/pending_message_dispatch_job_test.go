package job_test

import (
	"context"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/CodeMachine0121/go-trading/internal/job"
	"go.uber.org/mock/gomock"
)

func TestPendingMessageDispatchJobLooksAtTheQueueOnStartAndThenEveryInterval(t *testing.T) {
	mockController := gomock.NewController(t)
	looks := make(chan string, 64)
	pendingMessageRepository := mocks.NewMockIPendingMessageRepository(mockController)
	pendingMessageRepository.EXPECT().DeleteSettledBefore(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	pendingMessageRepository.EXPECT().FindUnsettled(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, int) ([]entities.PendingMessage, error) {
			looks <- "look"

			return []entities.PendingMessage{}, nil
		}).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(currentTime).AnyTimes()
	dispatchJob := job.NewPendingMessageDispatchJob(application.NewPendingMessageDispatchApplication(
		service.NewPendingMessageService(pendingMessageRepository, mocks.NewMockIStrategyBotRepository(mockController),
			mocks.NewMockITransactionRepository(mockController), nil, clockProxy, "replica-a", 2*time.Minute, 8)),
		testInterval)
	t.Cleanup(dispatchJob.Stop)

	dispatchJob.Start(t.Context())

	nextFrom(t, looks)
	nextFrom(t, looks)
}

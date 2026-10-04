package application_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"go.uber.org/mock/gomock"
)

// contractAutoOrderRepositoryNotInvolved stands in where no auto order is queued: history reads find none, and queuing one fails the test.
func contractAutoOrderRepositoryNotInvolved(t *testing.T) *mocks.MockIContractAutoOrderRepository {
	contractAutoOrderRepository := mocks.NewMockIContractAutoOrderRepository(gomock.NewController(t))
	contractAutoOrderRepository.EXPECT().FindByBotRunNumbers(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()

	return contractAutoOrderRepository
}

package job_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"go.uber.org/mock/gomock"
)

// noOrderFlowService is for tests whose scripts declare no order flow price step: its trade history and clock have no expectations, so any attempt to gather order flow fails the test.
func noOrderFlowService(t *testing.T) *service.ContractOrderFlowService {
	t.Helper()

	controller := gomock.NewController(t)

	return service.NewContractOrderFlowService(
		mocks.NewMockIContractTradeHistoryProxy(controller),
		mocks.NewMockIClockProxy(controller),
		service.NewContractOrderFlowMemory(1<<20))
}

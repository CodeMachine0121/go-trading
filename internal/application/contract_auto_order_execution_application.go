package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractAutoOrderExecutionApplication carries out queued auto orders; every replica runs it, and the queue makes sure each order is held by one at a time.
type ContractAutoOrderExecutionApplication struct {
	contractAutoOrderService *service.ContractAutoOrderService
}

func NewContractAutoOrderExecutionApplication(
	contractAutoOrderService *service.ContractAutoOrderService,
) *ContractAutoOrderExecutionApplication {
	return &ContractAutoOrderExecutionApplication{contractAutoOrderService: contractAutoOrderService}
}

func (contractAutoOrderExecutionApplication *ContractAutoOrderExecutionApplication) ExecuteDueAutoOrders(
	executionContext context.Context,
) (int, error) {
	return contractAutoOrderExecutionApplication.contractAutoOrderService.ExecuteDueAutoOrders(executionContext)
}

package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractPositionStatisticApplication is every use case about perpetual contract
// position statistics: recording them, and reading them back.
type ContractPositionStatisticApplication struct {
	contractPositionStatisticService *service.ContractPositionStatisticService
}

func NewContractPositionStatisticApplication(
	contractPositionStatisticService *service.ContractPositionStatisticService,
) *ContractPositionStatisticApplication {
	return &ContractPositionStatisticApplication{
		contractPositionStatisticService: contractPositionStatisticService,
	}
}

// RunRound records whatever is new for every watched contract.
func (positionStatisticApplication *ContractPositionStatisticApplication) RunRound(
	executionContext context.Context,
) (dto.ContractSeriesIngestionReportDto, error) {
	return positionStatisticApplication.contractPositionStatisticService.RunRound(executionContext)
}

// CatchUpSymbol records whatever is new for one registered contract.
func (positionStatisticApplication *ContractPositionStatisticApplication) CatchUpSymbol(
	executionContext context.Context, symbol string,
) (dto.ContractSeriesSymbolReportDto, error) {
	return positionStatisticApplication.contractPositionStatisticService.RunRoundFor(executionContext, symbol)
}

// GetStatisticsInRange reads one contract's statistics back, earliest first.
func (positionStatisticApplication *ContractPositionStatisticApplication) GetStatisticsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractPositionStatisticDto, error) {
	return positionStatisticApplication.contractPositionStatisticService.FindStatisticsInRange(
		executionContext, queryDto)
}

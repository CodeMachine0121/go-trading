package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

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

func (positionStatisticApplication *ContractPositionStatisticApplication) RunRound(
	executionContext context.Context,
) (dto.ContractSeriesIngestionReportDto, error) {
	return positionStatisticApplication.contractPositionStatisticService.RunRound(executionContext)
}

func (positionStatisticApplication *ContractPositionStatisticApplication) CatchUpSymbol(
	executionContext context.Context, symbol string,
) (dto.ContractSeriesSymbolReportDto, error) {
	return positionStatisticApplication.contractPositionStatisticService.RunRoundFor(executionContext, symbol)
}

// GetStatisticsInRange returns statistics earliest first.
func (positionStatisticApplication *ContractPositionStatisticApplication) GetStatisticsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractPositionStatisticDto, error) {
	return positionStatisticApplication.contractPositionStatisticService.FindStatisticsInRange(
		executionContext, queryDto)
}

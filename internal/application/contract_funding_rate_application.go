package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type ContractFundingRateApplication struct {
	contractFundingRateService *service.ContractFundingRateService
}

func NewContractFundingRateApplication(
	contractFundingRateService *service.ContractFundingRateService,
) *ContractFundingRateApplication {
	return &ContractFundingRateApplication{contractFundingRateService: contractFundingRateService}
}

func (contractFundingRateApplication *ContractFundingRateApplication) RunRound(
	executionContext context.Context,
) (dto.ContractSeriesIngestionReportDto, error) {
	return contractFundingRateApplication.contractFundingRateService.RunRound(executionContext)
}

func (contractFundingRateApplication *ContractFundingRateApplication) CatchUpSymbol(
	executionContext context.Context, symbol string,
) (dto.ContractSeriesSymbolReportDto, error) {
	return contractFundingRateApplication.contractFundingRateService.RunRoundFor(executionContext, symbol)
}

// GetSettlementsInRange returns settlements earliest first.
func (contractFundingRateApplication *ContractFundingRateApplication) GetSettlementsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractFundingRateSettlementDto, error) {
	return contractFundingRateApplication.contractFundingRateService.FindSettlementsInRange(
		executionContext, queryDto)
}

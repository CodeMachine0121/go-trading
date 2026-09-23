package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractFundingRateApplication is every use case about perpetual contract funding
// rate settlements: catching them up, and reading them back.
type ContractFundingRateApplication struct {
	contractFundingRateService *service.ContractFundingRateService
}

func NewContractFundingRateApplication(
	contractFundingRateService *service.ContractFundingRateService,
) *ContractFundingRateApplication {
	return &ContractFundingRateApplication{contractFundingRateService: contractFundingRateService}
}

// RunRound catches every watched contract up.
func (contractFundingRateApplication *ContractFundingRateApplication) RunRound(
	executionContext context.Context,
) (dto.ContractSeriesIngestionReportDto, error) {
	return contractFundingRateApplication.contractFundingRateService.RunRound(executionContext)
}

// CatchUpSymbol catches one registered contract up.
func (contractFundingRateApplication *ContractFundingRateApplication) CatchUpSymbol(
	executionContext context.Context, symbol string,
) (dto.ContractSeriesSymbolReportDto, error) {
	return contractFundingRateApplication.contractFundingRateService.RunRoundFor(executionContext, symbol)
}

// GetSettlementsInRange reads one contract's settlements back, earliest first.
func (contractFundingRateApplication *ContractFundingRateApplication) GetSettlementsInRange(
	executionContext context.Context, queryDto dto.KCandleQueryDto,
) ([]dto.ContractFundingRateSettlementDto, error) {
	return contractFundingRateApplication.contractFundingRateService.FindSettlementsInRange(
		executionContext, queryDto)
}

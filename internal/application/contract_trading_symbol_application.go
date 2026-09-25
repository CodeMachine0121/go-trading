package application

import (
	"context"
	"errors"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type ContractTradingSymbolApplication struct {
	contractTradingSymbolService     *service.ContractTradingSymbolService
	contractKCandleIngestionService  *service.ContractKCandleIngestionService
	contractFundingRateService       *service.ContractFundingRateService
	contractPositionStatisticService *service.ContractPositionStatisticService
	maintenanceMarginTierService     *service.ContractMaintenanceMarginTierService
}

func NewContractTradingSymbolApplication(
	contractTradingSymbolService *service.ContractTradingSymbolService,
	contractKCandleIngestionService *service.ContractKCandleIngestionService,
	contractFundingRateService *service.ContractFundingRateService,
	contractPositionStatisticService *service.ContractPositionStatisticService,
	maintenanceMarginTierService *service.ContractMaintenanceMarginTierService,
) *ContractTradingSymbolApplication {
	return &ContractTradingSymbolApplication{
		contractTradingSymbolService:     contractTradingSymbolService,
		contractKCandleIngestionService:  contractKCandleIngestionService,
		contractFundingRateService:       contractFundingRateService,
		contractPositionStatisticService: contractPositionStatisticService,
		maintenanceMarginTierService:     maintenanceMarginTierService,
	}
}

func (contractTradingSymbolApplication *ContractTradingSymbolApplication) ListContractTradingSymbols(
	executionContext context.Context,
) ([]dto.ContractTradingSymbolDto, error) {
	return contractTradingSymbolApplication.contractTradingSymbolService.
		ListContractTradingSymbols(executionContext)
}

// AddToWatchlist also catches the contract up at once, sequentially since the series share a venue allowance; position statistics can't wait because the venue forgets them after thirty days.
// A failed catch-up does not fail the add, since the add itself succeeded and regular rounds will catch up.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) AddToWatchlist(
	executionContext context.Context, symbol string,
) error {
	if addError := contractTradingSymbolApplication.contractTradingSymbolService.AddToWatchlist(
		executionContext, symbol); addError != nil {
		return addError
	}

	if _, catchUpError := contractTradingSymbolApplication.contractKCandleIngestionService.
		RunBackfillFor(executionContext, symbol); catchUpError != nil {
		log.Printf("contract watchlist: %s was added but its candles could not be caught up: %v",
			symbol, catchUpError)
	}

	fundingRateReport, fundingRateError := contractTradingSymbolApplication.contractFundingRateService.
		RunRoundFor(executionContext, symbol)
	contractTradingSymbolApplication.noteCatchUpFailure(symbol, "funding rates", fundingRateReport, fundingRateError)

	positionStatisticReport, positionStatisticError := contractTradingSymbolApplication.
		contractPositionStatisticService.RunRoundFor(executionContext, symbol)
	contractTradingSymbolApplication.noteCatchUpFailure(
		symbol, "position statistics", positionStatisticReport, positionStatisticError)

	// The venue returns every contract's ladder at once, so refresh them all; without an account there is nothing to ask.
	marginReport, marginError := contractTradingSymbolApplication.maintenanceMarginTierService.
		RefreshLadders(executionContext)
	switch {
	case errors.Is(marginError, domains.ErrContractAccountCredentialsMissing):
	case marginError != nil:
		log.Printf("contract watchlist: %s was added but its maintenance margin ladder could not be fetched: %v",
			symbol, marginError)
	default:
		for _, refusedLadder := range marginReport.RefusedLadders {
			log.Printf("contract watchlist: %s was added but the maintenance margin ladder of %s was kept as it was: %s",
				symbol, refusedLadder.Symbol, refusedLadder.Reason)
		}
	}

	return nil
}

func (contractTradingSymbolApplication *ContractTradingSymbolApplication) RefreshTradingSpecifications(
	executionContext context.Context,
) (int, error) {
	return contractTradingSymbolApplication.contractTradingSymbolService.
		RefreshTradingSpecifications(executionContext)
}

// RemoveFromWatchlist keeps all data already stored for the contract.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	return contractTradingSymbolApplication.contractTradingSymbolService.RemoveFromWatchlist(
		executionContext, symbol)
}

// noteCatchUpFailure logs both an outright error and a report of venue/storage failures, since a round reports per-contract failures instead of erroring.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) noteCatchUpFailure(
	symbol string, seriesName string, symbolReport dto.ContractSeriesSymbolReportDto, catchUpError error,
) {
	if catchUpError != nil {
		log.Printf("contract watchlist: %s was added but its %s could not be caught up: %v",
			symbol, seriesName, catchUpError)

		return
	}

	if symbolReport.FetchFailureReason != "" {
		log.Printf("contract watchlist: %s was added but its %s could not be caught up: %s",
			symbol, seriesName, symbolReport.FetchFailureReason)
	}
}

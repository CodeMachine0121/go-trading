package application

import (
	"context"
	"errors"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractTradingSymbolApplication orchestrates the perpetual contract watchlist use
// cases.
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

// ListContractTradingSymbols returns every perpetual contract the system knows about.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) ListContractTradingSymbols(
	executionContext context.Context,
) ([]dto.ContractTradingSymbolDto, error) {
	return contractTradingSymbolApplication.contractTradingSymbolService.
		ListContractTradingSymbols(executionContext)
}

// AddToWatchlist starts keeping one perpetual contract's data up to date, and catches
// that contract up on the spot: its candles, its whole funding rate history, the last
// thirty days of its position statistics, and — with an account configured — its full
// maintenance margin ladder.
//
// Without the second half a contract added now would hold no candles until the next
// start-up's backfill, because the scheduled round only ever collects what has closed
// since it last ran. The position statistics are the ones that cannot wait at all:
// the venue forgets them after thirty days.
//
// The three are caught up one after another rather than at once. Two of them spend
// the same venue allowance, so running them side by side would only queue them
// behind each other, and a person adding a contract waits for all three either way.
//
// **A failed catch-up does not fail the add.** The contract is on the watchlist — that
// is what was asked for and it is true — and the ordinary rounds will reach it anyway;
// refusing the add would undo something that already succeeded in order to report
// something that will fix itself.
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

	// The venue answers about every contract's ladder at once, so the new one is
	// caught up by refreshing them all — one question either way. Without an account
	// there is nothing to ask and nothing worth saying.
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

// RefreshTradingSpecifications brings every known contract's trading specification up
// to date, and says how many it updated.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) RefreshTradingSpecifications(
	executionContext context.Context,
) (int, error) {
	return contractTradingSymbolApplication.contractTradingSymbolService.
		RefreshTradingSpecifications(executionContext)
}

// RemoveFromWatchlist stops keeping one perpetual contract's data up to date, leaving
// every candle, funding rate settlement and position statistic it already holds
// exactly where it is.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	return contractTradingSymbolApplication.contractTradingSymbolService.RemoveFromWatchlist(
		executionContext, symbol)
}

// noteCatchUpFailure writes down a series that could not be caught up when a contract
// joined the watchlist. There are two ways it can go wrong, and both have to be said
// out loud: the catch-up refusing outright, and the catch-up running while the venue
// or storage would not answer — which comes back as a report, not an error, because a
// round never fails for one contract. Left unsaid, the second looks exactly like a
// catch-up that found nothing.
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

package application

import (
	"context"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// ContractTradingSymbolApplication orchestrates the perpetual contract watchlist use
// cases.
type ContractTradingSymbolApplication struct {
	contractTradingSymbolService    *service.ContractTradingSymbolService
	contractKCandleIngestionService *service.ContractKCandleIngestionService
}

func NewContractTradingSymbolApplication(
	contractTradingSymbolService *service.ContractTradingSymbolService,
	contractKCandleIngestionService *service.ContractKCandleIngestionService,
) *ContractTradingSymbolApplication {
	return &ContractTradingSymbolApplication{
		contractTradingSymbolService:    contractTradingSymbolService,
		contractKCandleIngestionService: contractKCandleIngestionService,
	}
}

// ListContractTradingSymbols returns every perpetual contract the system knows about.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) ListContractTradingSymbols(
	executionContext context.Context,
) ([]dto.ContractTradingSymbolDto, error) {
	return contractTradingSymbolApplication.contractTradingSymbolService.
		ListContractTradingSymbols(executionContext)
}

// AddToWatchlist starts keeping one perpetual contract's candles up to date, and
// catches that contract up on the spot.
//
// Without the second half a contract added now would hold nothing until the next
// start-up's backfill, because the scheduled round only ever collects what has closed
// since it last ran.
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
		log.Printf("contract watchlist: %s was added but could not be caught up: %v",
			symbol, catchUpError)
	}

	return nil
}

// RemoveFromWatchlist stops keeping one perpetual contract's candles up to date,
// leaving every candle it already holds exactly where it is.
// RefreshTradingSpecifications brings every known contract's trading specification up
// to date, and says how many it updated.
func (contractTradingSymbolApplication *ContractTradingSymbolApplication) RefreshTradingSpecifications(
	executionContext context.Context,
) (int, error) {
	return contractTradingSymbolApplication.contractTradingSymbolService.
		RefreshTradingSpecifications(executionContext)
}

func (contractTradingSymbolApplication *ContractTradingSymbolApplication) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	return contractTradingSymbolApplication.contractTradingSymbolService.RemoveFromWatchlist(
		executionContext, symbol)
}

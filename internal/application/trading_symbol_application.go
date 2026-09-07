package application

import (
	"context"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// TradingSymbolApplication orchestrates the trading symbol use cases.
//
// Adding to the watchlist is the one place it sequences two domain services rather
// than calling one — which is exactly what this layer is for, and why neither service
// knows about the other.
type TradingSymbolApplication struct {
	tradingSymbolService    *service.TradingSymbolService
	kCandleIngestionService *service.KCandleIngestionService
}

func NewTradingSymbolApplication(
	tradingSymbolService *service.TradingSymbolService,
	kCandleIngestionService *service.KCandleIngestionService,
) *TradingSymbolApplication {
	return &TradingSymbolApplication{
		tradingSymbolService:    tradingSymbolService,
		kCandleIngestionService: kCandleIngestionService,
	}
}

func (tradingSymbolApplication *TradingSymbolApplication) ListTradingSymbols(
	executionContext context.Context,
) ([]dto.TradingSymbolDto, error) {
	return tradingSymbolApplication.tradingSymbolService.ListTradingSymbols(executionContext)
}

// RegisterDefaultTradingSymbols reports which markets this run newly registered.
func (tradingSymbolApplication *TradingSymbolApplication) RegisterDefaultTradingSymbols(
	executionContext context.Context,
) ([]string, error) {
	return tradingSymbolApplication.tradingSymbolService.RegisterDefaultTradingSymbols(executionContext)
}

// AddToWatchlist starts keeping one market's candles up to date, and catches that
// symbol up on the spot.
//
// Without the second half, a symbol added after its market shut would hold no candles
// at all until the next start-up: the scheduled round only ever collects what has
// closed since it last ran, and after the bell there is nothing left to collect. So
// somebody adding a stock in the evening would be looking at an empty chart with
// nothing they could do about it.
//
// A failed catch-up does not fail the add. The symbol is on the watchlist — that is
// what was asked for and it is true — and the ordinary rounds will reach it anyway;
// refusing the add would undo something that already succeeded to report something
// that will fix itself.
func (tradingSymbolApplication *TradingSymbolApplication) AddToWatchlist(
	executionContext context.Context, entryDto dto.WatchlistEntryDto,
) error {
	if addError := tradingSymbolApplication.tradingSymbolService.AddToWatchlist(
		executionContext, entryDto); addError != nil {
		return addError
	}

	if _, catchUpError := tradingSymbolApplication.kCandleIngestionService.RunBackfillFor(
		executionContext, entryDto.Symbol); catchUpError != nil {
		log.Printf("watchlist: %s was added but could not be caught up: %v",
			entryDto.Symbol, catchUpError)
	}

	return nil
}

// RemoveFromWatchlist stops keeping one market's candles up to date, leaving every
// candle it already holds exactly where it is.
func (tradingSymbolApplication *TradingSymbolApplication) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	return tradingSymbolApplication.tradingSymbolService.RemoveFromWatchlist(executionContext, symbol)
}

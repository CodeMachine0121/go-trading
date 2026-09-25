package application

import (
	"context"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

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

func (tradingSymbolApplication *TradingSymbolApplication) RegisterDefaultTradingSymbols(
	executionContext context.Context,
) ([]string, error) {
	return tradingSymbolApplication.tradingSymbolService.RegisterDefaultTradingSymbols(executionContext)
}

// AddToWatchlist also backfills the symbol immediately, because scheduled rounds only collect newly
// closed candles and a symbol added after market close would otherwise stay empty until restart; a
// failed backfill is only logged since the add itself succeeded.
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

// RemoveFromWatchlist stops ingestion but keeps the candles already stored.
func (tradingSymbolApplication *TradingSymbolApplication) RemoveFromWatchlist(
	executionContext context.Context, symbol string,
) error {
	return tradingSymbolApplication.tradingSymbolService.RemoveFromWatchlist(executionContext, symbol)
}

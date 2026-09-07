package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_trading_symbol_repository.go -destination=mocks/mock_i_trading_symbol_repository.go -package=mocks

// ITradingSymbolRepository stores and retrieves the markets the system knows about.
type ITradingSymbolRepository interface {
	// FindAll returns every registered trading symbol, ordered by name.
	FindAll(executionContext context.Context) ([]entities.TradingSymbol, error)
	// FindWatched returns the trading symbols the system is keeping up to date,
	// earliest registered first — the order a market with a follow ceiling hands out
	// its places in. Ties are settled by name so the order never wobbles.
	FindWatched(executionContext context.Context) ([]entities.TradingSymbol, error)
	// FindBySymbol returns one registered trading symbol, reporting whether it was
	// registered at all. Not being registered is an answer rather than a failure: it
	// is what "the system does not know this market" looks like.
	FindBySymbol(executionContext context.Context, symbol string) (entities.TradingSymbol, bool, error)
	// Save stores one trading symbol, replacing whatever was held under the same
	// name. Registering a market twice leaves one market, which is what makes adding
	// something already on the watchlist a no-op rather than a failure.
	Save(executionContext context.Context, tradingSymbol entities.TradingSymbol) error
	// RegisterAll stores the given trading symbols. Registering one that is already
	// registered changes nothing and is not an error — two migrations running at once
	// must not fail over which of them got there first.
	RegisterAll(executionContext context.Context, tradingSymbols []entities.TradingSymbol) error
}

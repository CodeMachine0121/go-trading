package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_trading_symbol_repository.go -destination=mocks/mock_i_trading_symbol_repository.go -package=mocks

// ITradingSymbolRepository stores and retrieves the markets the system knows about.
type ITradingSymbolRepository interface {
	// FindAll is ordered by name.
	FindAll(executionContext context.Context) ([]entities.TradingSymbol, error)
	// FindWatched is ordered by registration time (how follow-limited markets allocate places), ties broken by name.
	FindWatched(executionContext context.Context) ([]entities.TradingSymbol, error)
	FindBySymbol(executionContext context.Context, symbol string) (entities.TradingSymbol, bool, error)
	// Save replaces any symbol with the same name, making re-adding a no-op.
	Save(executionContext context.Context, tradingSymbol entities.TradingSymbol) error
	// RegisterAll ignores already-registered symbols so concurrent migrations do not fail.
	RegisterAll(executionContext context.Context, tradingSymbols []entities.TradingSymbol) error
}

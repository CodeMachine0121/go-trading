package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

//go:generate go tool mockgen -source=i_trading_symbol_repository.go -destination=mocks/mock_i_trading_symbol_repository.go -package=mocks

// ITradingSymbolRepository stores and retrieves the markets the system knows about.
type ITradingSymbolRepository interface {
	// FindAll returns every registered trading symbol, ordered by name.
	FindAll() ([]entities.TradingSymbol, error)
	// RegisterAll stores the given trading symbols. Registering one that is already
	// registered changes nothing and is not an error — two migrations running at once
	// must not fail over which of them got there first.
	RegisterAll(tradingSymbols []entities.TradingSymbol) error
}

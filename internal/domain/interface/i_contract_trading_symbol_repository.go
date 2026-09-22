package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_trading_symbol_repository.go -destination=mocks/mock_i_contract_trading_symbol_repository.go -package=mocks

// IContractTradingSymbolRepository stores and retrieves the perpetual contracts the
// system knows about.
//
// It is kept apart from ITradingSymbolRepository because the two lists have to be
// able to hold the same name at once: BTCUSDT exists on both venues and is a
// different instrument on each.
type IContractTradingSymbolRepository interface {
	FindAll(executionContext context.Context) ([]entities.ContractTradingSymbol, error)
	// FindWatched returns only the contracts the automatic round has to keep up to
	// date. It is read afresh at the start of every round, which is what lets a change
	// take effect within one round rather than at the next restart.
	FindWatched(executionContext context.Context) ([]entities.ContractTradingSymbol, error)
	FindBySymbol(
		executionContext context.Context, symbol string,
	) (entities.ContractTradingSymbol, bool, error)
	Save(executionContext context.Context, contractTradingSymbol entities.ContractTradingSymbol) error
}

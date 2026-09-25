package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_trading_symbol_repository.go -destination=mocks/mock_i_contract_trading_symbol_repository.go -package=mocks

// IContractTradingSymbolRepository is separate from ITradingSymbolRepository because the same name (e.g. BTCUSDT) is a different instrument on each venue.
type IContractTradingSymbolRepository interface {
	FindAll(executionContext context.Context) ([]entities.ContractTradingSymbol, error)
	// FindWatched is re-read every round so watch changes take effect without a restart.
	FindWatched(executionContext context.Context) ([]entities.ContractTradingSymbol, error)
	FindBySymbol(
		executionContext context.Context, symbol string,
	) (entities.ContractTradingSymbol, bool, error)
	// Save leaves the stored trading specification unchanged when the contract carries none.
	Save(executionContext context.Context, contractTradingSymbol entities.ContractTradingSymbol) error
	// SaveTradingSpecifications writes only specifications, leaving watch state alone, and never creates unregistered contracts.
	SaveTradingSpecifications(
		executionContext context.Context, contractTradingSymbols []entities.ContractTradingSymbol,
	) error
}

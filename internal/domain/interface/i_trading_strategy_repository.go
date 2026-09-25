package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_trading_strategy_repository.go -destination=mocks/mock_i_trading_strategy_repository.go -package=mocks

// ITradingStrategyRepository stores strategies whole; sources and condition nodes never live on their own, so rules are never half-rewritten.
type ITradingStrategyRepository interface {
	// Save creates when the identifier is zero and replaces the whole strategy otherwise.
	Save(
		executionContext context.Context, tradingStrategy entities.TradingStrategy,
	) (entities.TradingStrategy, error)

	// FindOne returns sources and both trees; ownership checks are the domain's job.
	FindOne(executionContext context.Context, id uint) (entities.TradingStrategy, error)

	FindAllByOwner(
		executionContext context.Context, ownerID uint,
	) ([]entities.TradingStrategy, error)

	// Delete removes sources and condition nodes by cascade.
	Delete(executionContext context.Context, id uint) error
}

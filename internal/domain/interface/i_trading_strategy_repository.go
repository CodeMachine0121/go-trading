package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_trading_strategy_repository.go -destination=mocks/mock_i_trading_strategy_repository.go -package=mocks

// ITradingStrategyRepository stores trading strategies whole.
//
// There is no repository for signal sources or for condition nodes, and that is
// deliberate: neither is ever read, created or deleted on its own. They arrive and
// leave with the trading strategy that owns them, which is what stops a set of rules
// ever existing half-rewritten — sources replaced, conditions not, naming a label
// that no longer exists.
type ITradingStrategyRepository interface {
	// Save stores this trading strategy whole, replacing whatever it had before,
	// and hands back what was stored. A create is a zero identifier; a rewrite
	// names one.
	Save(
		executionContext context.Context, tradingStrategy entities.TradingStrategy,
	) (entities.TradingStrategy, error)

	// FindOne returns it with its sources and both trees, or the not-found refusal.
	// It does not ask who wants it: whether this is the caller's is a question the
	// domain answers, and answering it here would mean answering it in every
	// implementation.
	FindOne(executionContext context.Context, id uint) (entities.TradingStrategy, error)

	// FindAllByOwner returns this person's trading strategies, by name.
	FindAllByOwner(
		executionContext context.Context, ownerID uint,
	) ([]entities.TradingStrategy, error)

	// Delete removes it. Its sources and condition nodes go with it by cascade.
	Delete(executionContext context.Context, id uint) error
}

package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_binance_trading_key_repository.go -destination=mocks/mock_i_binance_trading_key_repository.go -package=mocks

// IBinanceTradingKeyRepository keys every method by user; both writes also switch auto order off in the same transaction, so no bot is ever left on without a key covering it.
type IBinanceTradingKeyRepository interface {
	// FindOneByUser returns ErrBinanceTradingKeyNotConfigured when none is stored.
	FindOneByUser(executionContext context.Context, userID uint) (entities.BinanceTradingKey, error)
	// Replace upserts the key on the unique user index and switches auto order off on the owner's bots of the given kinds.
	Replace(
		executionContext context.Context,
		binanceTradingKey entities.BinanceTradingKey,
		uncoveredBotMarketDataKinds []string,
	) (entities.BinanceTradingKey, error)
	// DeleteByUser switches auto order off on every bot of the user and is a no-op for the key when none is stored.
	DeleteByUser(executionContext context.Context, userID uint) error
}

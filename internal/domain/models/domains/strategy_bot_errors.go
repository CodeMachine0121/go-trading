package domains

import (
	"errors"
	"fmt"
)

// ErrStrategyBotValidation is the single sentinel for every bot rule violation; the wrapped message names the rule.
var ErrStrategyBotValidation = errors.New("strategy bot validation failed")

// ErrStrategyBotNotFound also covers bots owned by someone else so identifiers can't be probed for existence.
var ErrStrategyBotNotFound = errors.New("strategy bot not found")

func StrategyBotNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的策略機器人", ErrStrategyBotNotFound, id)
}

// ErrStrategyBotNameConflict is scoped to one owner; other owners may reuse a name.
var ErrStrategyBotNameConflict = errors.New("strategy bot name already in use")

// ErrStrategyBotRunning refuses edits mid-run so every round uses a well-defined version of the bot.
var ErrStrategyBotRunning = errors.New("strategy bot is running")

var ErrStrategyBotDeliveryNotConfigured = errors.New("strategy bot delivery not configured")

// ErrStrategyBotRunningLimitReached never stops another bot to make room.
var ErrStrategyBotRunningLimitReached = errors.New("strategy bot running limit reached")

// ErrStrategyBotAlreadyRunningARound refuses rather than queues a manual round, since a queued one would re-read the same candles.
var ErrStrategyBotAlreadyRunningARound = errors.New("strategy bot is already running a round")

// StrategyBotAlreadyRunningARound names no identifier because the caller is already looking at the bot.
func StrategyBotAlreadyRunningARound() error {
	return fmt.Errorf(
		"%w: 這台機器人正在跑一輪，等它跑完再試一次", ErrStrategyBotAlreadyRunningARound)
}

// ErrStrategyBotMarketDataStale means a contract's candles stopped arriving (e.g. it left the watchlist); the round is skipped rather than halting the bot.
var ErrStrategyBotMarketDataStale = errors.New("strategy bot market data stale")

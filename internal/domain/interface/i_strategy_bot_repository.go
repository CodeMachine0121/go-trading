package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_bot_repository.go -destination=mocks/mock_i_strategy_bot_repository.go -package=mocks

// IStrategyBotRepository stores bots together with their sources, parameters and condition trees, which have no meaning apart from the bot.
// UpdateRunState is separate from Save so a finishing round can only touch run-state columns.
type IStrategyBotRepository interface {
	// Save replaces the whole bot (record, sources and both trees) in one call.
	Save(executionContext context.Context, bot entities.StrategyBot) (entities.StrategyBot, error)

	// FindOne maps storage not-found to the domain's not-found error.
	FindOne(executionContext context.Context, id uint) (entities.StrategyBot, error)

	FindAllByOwner(executionContext context.Context, ownerID uint) ([]entities.StrategyBot, error)

	// FindAllByTradingStrategy returns every bot using the strategy regardless of owner or state, in one read so both running-bot and any-bot refusals see the same moment.
	FindAllByTradingStrategy(
		executionContext context.Context, tradingStrategyID uint,
	) ([]entities.StrategyBot, error)

	// Delete removes sources and trees by cascade.
	Delete(executionContext context.Context, id uint) error

	// UpdateRunState writes only run state, next due time, last signal, halt reason and conflict flag.
	UpdateRunState(executionContext context.Context, bot entities.StrategyBot) error

	CountRunningByOwner(executionContext context.Context, ownerID uint) (int, error)

	// FindDue returns running bots due at or before moment, oldest due first to avoid starvation, capped by limit in the query itself.
	FindDue(
		executionContext context.Context, moment time.Time, limit int,
	) ([]entities.StrategyBot, error)
}

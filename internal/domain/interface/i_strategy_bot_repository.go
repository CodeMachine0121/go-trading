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

	// FindAllByStrategyScript returns every bot whose trading strategy has a signal source naming the script, regardless of owner or state, so a rewrite sees running and idle references at the same moment.
	FindAllByStrategyScript(
		executionContext context.Context, strategyScriptID uint,
	) ([]entities.StrategyBot, error)

	// Delete removes sources and trees by cascade.
	Delete(executionContext context.Context, id uint) error

	// UpdateRunState writes only run state, next due time, last signal, halt reason and conflict flag.
	UpdateRunState(executionContext context.Context, bot entities.StrategyBot) error

	CountRunningByOwner(executionContext context.Context, ownerID uint) (int, error)

	// EnableAutoOrder switches auto order on only while the owner's trading key is still the one configured at binanceTradingKeyConfiguredAt, holding that key row against replacement or removal until done; otherwise ErrStrategyBotAutoOrderKeyChanged.
	EnableAutoOrder(
		executionContext context.Context, id uint, ownerID uint, binanceTradingKeyConfiguredAt time.Time,
	) error

	DisableAutoOrder(executionContext context.Context, id uint) error

	// ClaimDue claims for claimant, until claimedUntil, up to limit running bots due at or before moment that no other replica holds a live claim on, oldest due first; rows another replica is claiming at this instant are skipped, never waited for.
	ClaimDue(
		executionContext context.Context, moment time.Time, limit int, claimant string, claimedUntil time.Time,
	) ([]entities.StrategyBot, error)

	// ClaimOne claims one bot regardless of its run state; false means another replica's claim is still live at moment.
	ClaimOne(
		executionContext context.Context, id uint, claimant string, moment time.Time, claimedUntil time.Time,
	) (bool, error)

	// ReleaseRoundClaim frees the bot only when claimant still holds it, so a late finisher never frees a newer claim.
	ReleaseRoundClaim(executionContext context.Context, id uint, claimant string) error
}

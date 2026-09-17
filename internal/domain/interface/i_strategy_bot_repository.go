package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_bot_repository.go -destination=mocks/mock_i_strategy_bot_repository.go -package=mocks

// IStrategyBotRepository stores strategy bots, each with its signal sources and its
// two condition trees.
//
// There is one repository for four records because three of them are never asked
// for on their own: a source, a parameter value and a condition node have no meaning
// apart from the bot they belong to, and are never read, created or deleted without
// it. Giving them repositories of their own would be opening three doors nobody
// should walk through.
//
// Saving and changing a run state are separate on purpose. A round finishing has no
// business rewriting a condition tree, and an interface where it could is an
// interface where one day it will — the columns a round may touch are exactly the
// ones UpdateRunState names.
type IStrategyBotRepository interface {
	// Save stores this bot whole: the record, its sources and both trees, replacing
	// whatever the bot had before. It is one call rather than four because a bot
	// with new conditions and old sources is not a state anybody meant.
	Save(executionContext context.Context, bot entities.StrategyBot) (entities.StrategyBot, error)

	// FindOne returns this bot with everything hanging off it. Not finding one is
	// reported as the domain's not-found, so that nobody outside has to recognise a
	// storage library's sentinel.
	FindOne(executionContext context.Context, id uint) (entities.StrategyBot, error)

	// FindAllByOwner returns this person's bots, by name.
	FindAllByOwner(executionContext context.Context, ownerID uint) ([]entities.StrategyBot, error)

	// FindAllByTradingStrategy returns every bot following this set of rules,
	// whoever owns it and whether or not it is running.
	//
	// One read rather than a count and a separate list of the running ones: both
	// refusals that use it — a rewrite blocked by a running bot, a delete blocked
	// by any bot — are about the same moment, and two reads can disagree about it.
	FindAllByTradingStrategy(
		executionContext context.Context, tradingStrategyID uint,
	) ([]entities.StrategyBot, error)

	// Delete removes this bot; its sources and trees go with it, by cascade rather
	// than by any code here remembering them.
	Delete(executionContext context.Context, id uint) error

	// UpdateRunState writes only the columns a bot's life touches: run state, when
	// it is next due, the last signal sent, why it halted, and whether it is
	// conflicting.
	UpdateRunState(executionContext context.Context, bot entities.StrategyBot) error

	// CountRunningByOwner is how many of this person's bots are running, for the
	// one question that has a limit attached to it.
	CountRunningByOwner(executionContext context.Context, ownerID uint) (int, error)

	// FindDue returns running bots whose next round is due at or before this
	// moment, oldest due first so that nothing starves, and at most this many. The
	// cap is on the read rather than on the loop that follows it: a system coming
	// back after a long stop would otherwise pull every bot it has into memory to
	// then run four of them.
	FindDue(
		executionContext context.Context, moment time.Time, limit int,
	) ([]entities.StrategyBot, error)
}

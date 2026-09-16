package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_bot_run_record_repository.go -destination=mocks/mock_i_strategy_bot_run_record_repository.go -package=mocks

// IStrategyBotRunRecordRepository stores what each bot's rounds came to.
//
// It has a repository of its own, unlike a bot's sources and conditions, because it
// is the one thing hanging off a bot that *is* read on its own: somebody opens one
// bot's history without wanting the bot rewritten, and a bot is read on every scan
// without wanting a hundred rounds of history dragged along with it.
type IStrategyBotRunRecordRepository interface {
	// Append records one round and drops whatever falls out of the window this bot
	// keeps. Numbering and trimming happen together because they are two halves of
	// one fact — how many rounds this bot remembers — and apart they would be two
	// places that could disagree about it.
	Append(
		executionContext context.Context, strategyBotID uint, ranAt time.Time, result string,
	) error

	// FindLatestByBot returns this bot's remembered rounds, newest first.
	FindLatestByBot(
		executionContext context.Context, strategyBotID uint,
	) ([]entities.StrategyBotRunRecord, error)
}

package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_bot_run_record_repository.go -destination=mocks/mock_i_strategy_bot_run_record_repository.go -package=mocks

// IStrategyBotRunRecordRepository has its own repository because run history is read independently of the bot.
type IStrategyBotRunRecordRepository interface {
	// Append numbers the record across all the bot's symbols, trims the record's symbol beyond its retention window in one operation, and returns the record's number.
	Append(executionContext context.Context, writeDto dto.StrategyBotRunRecordWriteDto) (int, error)

	// FindLatestByBot returns newest first, only the given symbol's when one is given.
	FindLatestByBot(
		executionContext context.Context, strategyBotID uint, symbol string,
	) ([]entities.StrategyBotRunRecord, error)

	// FindByJournalLinkIdentifier answers false for a blank identifier or a round no longer remembered.
	FindByJournalLinkIdentifier(
		executionContext context.Context, journalLinkIdentifier string,
	) (entities.StrategyBotRunRecord, bool, error)
}

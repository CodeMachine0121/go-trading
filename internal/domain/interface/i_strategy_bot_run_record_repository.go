package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_strategy_bot_run_record_repository.go -destination=mocks/mock_i_strategy_bot_run_record_repository.go -package=mocks

// IStrategyBotRunRecordRepository has its own repository because run history is read independently of the bot.
type IStrategyBotRunRecordRepository interface {
	// Append numbers the round and trims beyond the bot's retention window in one operation.
	Append(executionContext context.Context, writeDto dto.StrategyBotRunRecordWriteDto) error

	// FindLatestByBot returns newest first.
	FindLatestByBot(
		executionContext context.Context, strategyBotID uint,
	) ([]entities.StrategyBotRunRecord, error)
}

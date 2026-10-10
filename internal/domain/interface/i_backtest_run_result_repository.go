package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_backtest_run_result_repository.go -destination=mocks/mock_i_backtest_run_result_repository.go -package=mocks

type IBacktestRunResultRepository interface {
	Save(executionContext context.Context, result entities.BacktestRunResult) error
	FindOne(executionContext context.Context, backtestRunID uint) (entities.BacktestRunResult, bool, error)
}

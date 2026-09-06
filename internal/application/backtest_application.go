package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// BacktestApplication orchestrates the strategy backtest use case.
type BacktestApplication struct {
	backtestService *service.BacktestService
}

func NewBacktestApplication(backtestService *service.BacktestService) *BacktestApplication {
	return &BacktestApplication{backtestService: backtestService}
}

func (backtestApplication *BacktestApplication) RunBacktest(
	executionContext context.Context, requestDto dto.BacktestRequestDto,
) (dto.BacktestResultDto, error) {
	return backtestApplication.backtestService.RunBacktest(executionContext, requestDto)
}

package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_k_candle_contract_summary_day_repository.go -destination=mocks/mock_i_k_candle_contract_summary_day_repository.go -package=mocks

type IKCandleContractSummaryDayRepository interface {
	FindDaysInRange(
		executionContext context.Context,
		symbol string,
		interval vo.AggregationIntervalVo,
		startTime time.Time,
		endTime time.Time,
	) ([]entities.KCandleContractSummaryDay, error)
	SaveSummarized(
		executionContext context.Context, summaryDay entities.KCandleContractSummaryDay,
	) (bool, error)
}

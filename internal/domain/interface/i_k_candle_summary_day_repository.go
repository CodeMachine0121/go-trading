package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_k_candle_summary_day_repository.go -destination=mocks/mock_i_k_candle_summary_day_repository.go -package=mocks

// IKCandleSummaryDayRepository keeps spot summaries a whole UTC day at a time; minute writes invalidate a day on their own, inside the same transaction.
type IKCandleSummaryDayRepository interface {
	// FindDaysInRange reads day states and their summaries from one snapshot, so an invalidation cannot strip a summarized day of its summaries mid-read.
	FindDaysInRange(
		executionContext context.Context,
		symbol string,
		interval vo.AggregationIntervalVo,
		startTime time.Time,
		endTime time.Time,
	) ([]entities.KCandleSummaryDay, error)
	// SaveSummarized stores nothing and reports false once the day's revision moved past the one it was merged at.
	SaveSummarized(executionContext context.Context, summaryDay entities.KCandleSummaryDay) (bool, error)
}

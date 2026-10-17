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
	// FindDaysInRange returns every day state from the day of startTime up to endTime, each summarized day carrying its summaries of the interval in [startTime, endTime), read from one snapshot so an invalidation cannot leave a summarized day without its summaries.
	FindDaysInRange(
		executionContext context.Context,
		symbol string,
		interval vo.AggregationIntervalVo,
		startTime time.Time,
		endTime time.Time,
	) ([]entities.KCandleSummaryDay, error)
	// SaveSummarized stores the day only while its revision is still the one it was merged at, reporting false and storing nothing otherwise.
	SaveSummarized(executionContext context.Context, summaryDay entities.KCandleSummaryDay) (bool, error)
}

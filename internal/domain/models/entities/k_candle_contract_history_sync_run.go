package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleContractHistorySyncRun has its own table, so its IDs are unrelated to spot run IDs.
type KCandleContractHistorySyncRun struct {
	ID           uint   `gorm:"primaryKey"`
	Symbol       string `gorm:"type:text;not null;index:idx_k_candle_contract_history_sync_runs_symbol"`
	LookbackDays int    `gorm:"not null"`
	Status       string `gorm:"type:text;not null;index:idx_k_candle_contract_history_sync_runs_status"`

	TotalChunks     int `gorm:"not null"`
	CompletedChunks int `gorm:"not null"`
	StoredCount     int `gorm:"not null"`
	SkippedCount    int `gorm:"not null"`

	// FetchFailureReason is the source refusing and FailureReason is this system breaking;
	// they are never merged.
	FetchFailureReason string    `gorm:"type:text;not null;default:''"`
	FailureReason      string    `gorm:"type:text;not null;default:''"`
	StartedAt          time.Time `gorm:"not null"`
	FinishedAt         *time.Time

	// Position statistic progress is counted apart from candles and defaults to zero for
	// older rows.
	PositionStatisticTotalDays          int    `gorm:"not null;default:0"`
	PositionStatisticCompletedDays      int    `gorm:"not null;default:0"`
	PositionStatisticStoredCount        int    `gorm:"not null;default:0"`
	PositionStatisticSkippedCount       int    `gorm:"not null;default:0"`
	PositionStatisticFetchFailureReason string `gorm:"type:text;not null;default:''"`
}

func (kCandleContractHistorySyncRun KCandleContractHistorySyncRun) TableName() string {
	return "KCandleContractHistorySyncRuns"
}

// ToDto reuses the spot run's progress shape and adds position statistic progress beside it.
func (kCandleContractHistorySyncRun KCandleContractHistorySyncRun) ToDto() dto.KCandleContractHistorySyncRunDto {
	return dto.KCandleContractHistorySyncRunDto{KCandleHistorySyncRunDto: dto.KCandleHistorySyncRunDto{
		ID:                 kCandleContractHistorySyncRun.ID,
		Symbol:             kCandleContractHistorySyncRun.Symbol,
		LookbackDays:       kCandleContractHistorySyncRun.LookbackDays,
		Status:             string(vo.NewKCandleHistorySyncRunStatusVo(kCandleContractHistorySyncRun.Status)),
		TotalChunks:        kCandleContractHistorySyncRun.TotalChunks,
		CompletedChunks:    kCandleContractHistorySyncRun.CompletedChunks,
		StoredCount:        kCandleContractHistorySyncRun.StoredCount,
		SkippedCount:       kCandleContractHistorySyncRun.SkippedCount,
		FetchFailureReason: kCandleContractHistorySyncRun.FetchFailureReason,
		FailureReason:      kCandleContractHistorySyncRun.FailureReason,
		StartedAt:          kCandleContractHistorySyncRun.StartedAt.UTC(),
		FinishedAt:         kCandleContractHistorySyncRun.FinishedAt,
	}, PositionStatistic: dto.ContractPositionStatisticSyncProgressDto{
		TotalDays:          kCandleContractHistorySyncRun.PositionStatisticTotalDays,
		CompletedDays:      kCandleContractHistorySyncRun.PositionStatisticCompletedDays,
		StoredCount:        kCandleContractHistorySyncRun.PositionStatisticStoredCount,
		SkippedCount:       kCandleContractHistorySyncRun.PositionStatisticSkippedCount,
		FetchFailureReason: kCandleContractHistorySyncRun.PositionStatisticFetchFailureReason,
	}}
}

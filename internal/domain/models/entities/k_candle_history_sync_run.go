package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleHistorySyncRun is written when the request is accepted, so the row itself is the
// in-progress work that callers poll and a restart sweeps up.
type KCandleHistorySyncRun struct {
	ID     uint   `gorm:"primaryKey"`
	Symbol string `gorm:"type:text;not null;index:idx_k_candle_history_sync_runs_symbol"`
	// LookbackDays is kept so a finished run still states the requested stretch.
	LookbackDays int    `gorm:"not null"`
	Status       string `gorm:"type:text;not null;index:idx_k_candle_history_sync_runs_status"`
	// ReplicaName is the replica running the sync, so only that replica vanishing marks it interrupted.
	ReplicaName string `gorm:"size:255;not null;default:''"`
	// Progress is counted in chunks, the unit the run actually advances in.
	TotalChunks     int `gorm:"not null"`
	CompletedChunks int `gorm:"not null"`
	StoredCount     int `gorm:"not null"`
	SkippedCount    int `gorm:"not null"`
	// PresumedClosedDayCount defaults to zero so runs recorded before it existed read as none.
	PresumedClosedDayCount int `gorm:"not null;default:0"`
	// FetchFailureReason is the source refusing; FailureReason is this system breaking.
	FetchFailureReason string    `gorm:"type:text;not null;default:''"`
	FailureReason      string    `gorm:"type:text;not null;default:''"`
	StartedAt          time.Time `gorm:"not null"`
	// FinishedAt is nil while the run is still going.
	FinishedAt *time.Time
}

func (kCandleHistorySyncRun KCandleHistorySyncRun) TableName() string {
	return "KCandleHistorySyncRuns"
}

func (kCandleHistorySyncRun KCandleHistorySyncRun) ToDto() dto.KCandleHistorySyncRunDto {
	return dto.KCandleHistorySyncRunDto{
		ID:                     kCandleHistorySyncRun.ID,
		Symbol:                 kCandleHistorySyncRun.Symbol,
		LookbackDays:           kCandleHistorySyncRun.LookbackDays,
		Status:                 string(vo.NewKCandleHistorySyncRunStatusVo(kCandleHistorySyncRun.Status)),
		TotalChunks:            kCandleHistorySyncRun.TotalChunks,
		CompletedChunks:        kCandleHistorySyncRun.CompletedChunks,
		StoredCount:            kCandleHistorySyncRun.StoredCount,
		SkippedCount:           kCandleHistorySyncRun.SkippedCount,
		PresumedClosedDayCount: kCandleHistorySyncRun.PresumedClosedDayCount,
		FetchFailureReason:     kCandleHistorySyncRun.FetchFailureReason,
		FailureReason:          kCandleHistorySyncRun.FailureReason,
		StartedAt:              kCandleHistorySyncRun.StartedAt.UTC(),
		FinishedAt:             kCandleHistorySyncRun.FinishedAt,
	}
}

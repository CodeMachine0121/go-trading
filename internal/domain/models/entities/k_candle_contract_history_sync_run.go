package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleContractHistorySyncRun is one contract history sync: what was asked for,
// where it has got to, and what it has collected so far. It is a plain data model —
// fields and persistence mapping only, no business rules.
//
// It is its own table, and therefore its own run of identifiers, rather than a column
// on the spot runs. The two numbering streams stay unrelated: whoever holds a run
// number has to know which kind of sync it belongs to, which is the price of the spot
// side needing no change at all.
type KCandleContractHistorySyncRun struct {
	ID           uint   `gorm:"primaryKey"`
	Symbol       string `gorm:"type:text;not null;index:idx_k_candle_contract_history_sync_runs_symbol"`
	LookbackDays int    `gorm:"not null"`
	Status       string `gorm:"type:text;not null;index:idx_k_candle_contract_history_sync_runs_status"`

	TotalChunks     int `gorm:"not null"`
	CompletedChunks int `gorm:"not null"`
	StoredCount     int `gorm:"not null"`
	SkippedCount    int `gorm:"not null"`

	// FetchFailureReason is the source refusing, which stops this run without being
	// this system's fault. FailureReason is this system having broken. The two ask
	// different things of the reader, so they are never merged.
	FetchFailureReason string    `gorm:"type:text;not null;default:''"`
	FailureReason      string    `gorm:"type:text;not null;default:''"`
	StartedAt          time.Time `gorm:"not null"`
	FinishedAt         *time.Time
}

// TableName pins the table to KCandleContractHistorySyncRuns instead of GORM's default.
func (kCandleContractHistorySyncRun KCandleContractHistorySyncRun) TableName() string {
	return "KCandleContractHistorySyncRuns"
}

// ToDto is the shape this run leaves the domain in.
//
// It reuses the shape the spot runs leave in, because progress is progress: what the
// caller cannot find out any other way is how far along the run is, and that question
// does not change with the kind of candle being fetched.
func (kCandleContractHistorySyncRun KCandleContractHistorySyncRun) ToDto() dto.KCandleHistorySyncRunDto {
	return dto.KCandleHistorySyncRunDto{
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
	}
}

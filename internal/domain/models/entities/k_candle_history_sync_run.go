package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleHistorySyncRun is one history sync: what was asked for, where it has got to,
// and what it has collected so far. It is a plain data model — fields and persistence
// mapping only, no business rules.
//
// The row is written when the request is accepted, not when the fetch finishes. A
// stretch of years takes minutes to hours, and the work exists nowhere else — not in
// a queue, not in a list somebody keeps in memory — so this row **is** the work in
// progress. That is what lets the caller come back and look, and what lets a restart
// sweep up after one it cut off.
type KCandleHistorySyncRun struct {
	ID     uint   `gorm:"primaryKey"`
	Symbol string `gorm:"type:text;not null;index:idx_k_candle_history_sync_runs_symbol"`
	// LookbackDays is what was asked for, kept so that a finished run still says what
	// stretch it covered. Working it back out of the timestamps would be a guess.
	LookbackDays int    `gorm:"not null"`
	Status       string `gorm:"type:text;not null;index:idx_k_candle_history_sync_runs_status"`
	// TotalChunks and CompletedChunks are the progress. They are counted in chunks
	// rather than in candles or days because a chunk is the unit the run actually
	// advances in, and a number that only moves when a whole day of fetching lands is
	// a number somebody can watch.
	TotalChunks     int `gorm:"not null"`
	CompletedChunks int `gorm:"not null"`
	StoredCount     int `gorm:"not null"`
	SkippedCount    int `gorm:"not null"`
	// FetchFailureReason is the source refusing, which stops this run without being
	// this system's fault. It is separate from FailureReason because the two ask
	// different things of the reader: one is somebody else's outage, the other is
	// ours.
	FetchFailureReason string `gorm:"type:text;not null;default:''"`
	// FailureReason is the one sentence a person reads when a run ended early, and is
	// empty on every other row.
	FailureReason string    `gorm:"type:text;not null;default:''"`
	StartedAt     time.Time `gorm:"not null"`
	// FinishedAt is nil while the run is still going. Nil and "not finished" are the
	// same thing here, which is why it is a pointer rather than a zero time nobody
	// can tell apart from an unset one.
	FinishedAt *time.Time
}

// TableName pins the table to KCandleHistorySyncRuns instead of GORM's default.
func (kCandleHistorySyncRun KCandleHistorySyncRun) TableName() string {
	return "KCandleHistorySyncRuns"
}

// ToDto is the shape this run leaves the domain in.
func (kCandleHistorySyncRun KCandleHistorySyncRun) ToDto() dto.KCandleHistorySyncRunDto {
	return dto.KCandleHistorySyncRunDto{
		ID:                 kCandleHistorySyncRun.ID,
		Symbol:             kCandleHistorySyncRun.Symbol,
		LookbackDays:       kCandleHistorySyncRun.LookbackDays,
		Status:             string(vo.NewKCandleHistorySyncRunStatusVo(kCandleHistorySyncRun.Status)),
		TotalChunks:        kCandleHistorySyncRun.TotalChunks,
		CompletedChunks:    kCandleHistorySyncRun.CompletedChunks,
		StoredCount:        kCandleHistorySyncRun.StoredCount,
		SkippedCount:       kCandleHistorySyncRun.SkippedCount,
		FetchFailureReason: kCandleHistorySyncRun.FetchFailureReason,
		FailureReason:      kCandleHistorySyncRun.FailureReason,
		StartedAt:          kCandleHistorySyncRun.StartedAt.UTC(),
		FinishedAt:         kCandleHistorySyncRun.FinishedAt,
	}
}

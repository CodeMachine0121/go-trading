package dto

import "time"

// KCandleHistorySyncRunDto reports progress, not candles, which go straight into storage.
type KCandleHistorySyncRunDto struct {
	ID              uint   `json:"id"`
	Symbol          string `json:"symbol"`
	LookbackDays    int    `json:"lookbackDays"`
	Status          string `json:"status"`
	TotalChunks     int    `json:"totalChunks"`
	CompletedChunks int    `json:"completedChunks"`
	StoredCount     int    `json:"storedCount"`
	SkippedCount    int    `json:"skippedCount"`
	// FetchFailureReason is the source refusing, which ends the run without counting as a
	// run failure.
	FetchFailureReason string `json:"fetchFailureReason"`
	// FailureReason is set only when this system broke.
	FailureReason string     `json:"failureReason"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

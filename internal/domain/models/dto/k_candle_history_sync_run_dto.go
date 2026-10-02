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
	// PresumedClosedDayCount is whole days the source said it holds nothing for, unlike
	// SkippedCount, which is candles the source sent but that failed their own rules.
	PresumedClosedDayCount int `json:"presumedClosedDayCount"`
	// FetchFailureReason is the source refusing, which ends the run without counting as a
	// run failure.
	FetchFailureReason string `json:"fetchFailureReason"`
	// FailureReason is set only when this system broke.
	FailureReason string     `json:"failureReason"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

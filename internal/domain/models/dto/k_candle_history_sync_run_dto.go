package dto

import "time"

// KCandleHistorySyncRunDto is what somebody who asked for a stretch of history gets
// back when they come to look: where the run has got to, and what it has collected.
//
// It answers with progress rather than with candles. The candles went into storage,
// which is where they were wanted; what the caller cannot find out any other way is
// whether the run is still going, and how much of the stretch is behind it.
type KCandleHistorySyncRunDto struct {
	ID           uint   `json:"id"`
	Symbol       string `json:"symbol"`
	LookbackDays int    `json:"lookbackDays"`
	Status       string `json:"status"`
	// TotalChunks and CompletedChunks are how far along it is. Both are chunks —
	// stretches of the whole the run advances in one at a time — so the pair reads as
	// "this many of that many" without needing a percentage nobody agreed on.
	TotalChunks     int `json:"totalChunks"`
	CompletedChunks int `json:"completedChunks"`
	StoredCount     int `json:"storedCount"`
	SkippedCount    int `json:"skippedCount"`
	// FetchFailureReason is the source having refused. A run that stops this way did
	// everything right and found out something about the market, so it is reported
	// here rather than as a failure of the run.
	FetchFailureReason string `json:"fetchFailureReason"`
	// FailureReason is this system having broken. It is empty on every other run.
	FailureReason string     `json:"failureReason"`
	StartedAt     time.Time  `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

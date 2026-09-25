package dto

// ContractPositionStatisticSyncProgressDto is how far a contract history sync has got
// with the position statistics, kept apart from how far it has got with the candles.
//
// **The two are never added together.** A stored count that meant candles and
// statistics at once would answer neither "did the candles come back" nor "did the
// statistics", and those are the two questions somebody looking at a run is asking.
type ContractPositionStatisticSyncProgressDto struct {
	// TotalDays and CompletedDays are days — the unit the venue's archive keeps
	// statistics in — so the pair reads as "this many of that many".
	TotalDays     int `json:"totalDays"`
	CompletedDays int `json:"completedDays"`
	StoredCount   int `json:"storedCount"`
	SkippedCount  int `json:"skippedCount"`
	// FetchFailureReason is the archive having refused, or having answered with
	// something that could not be read. It stops the statistics and nothing else,
	// and it is not a failure of the run.
	FetchFailureReason string `json:"fetchFailureReason"`
}

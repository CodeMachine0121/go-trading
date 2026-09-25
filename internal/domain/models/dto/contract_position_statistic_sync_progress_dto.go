package dto

// ContractPositionStatisticSyncProgressDto is tracked separately from candle progress and
// never summed with it.
type ContractPositionStatisticSyncProgressDto struct {
	// TotalDays and CompletedDays count days, the unit the venue archive stores statistics in.
	TotalDays     int `json:"totalDays"`
	CompletedDays int `json:"completedDays"`
	StoredCount   int `json:"storedCount"`
	SkippedCount  int `json:"skippedCount"`
	// FetchFailureReason stops only the statistics and does not fail the run.
	FetchFailureReason string `json:"fetchFailureReason"`
}

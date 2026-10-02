package dto

// InterruptedWorkDto counts work found cut off because the replica doing it is gone.
type InterruptedWorkDto struct {
	Answers              int
	HistorySyncs         int
	ContractHistorySyncs int
}

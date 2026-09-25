package dto

// KCandleContractHistorySyncRunDto is what somebody who asked for a stretch of a
// perpetual contract's history gets back: the same progress the spot runs report,
// and beside it how far the run has got with the position statistics.
//
// The spot shape is embedded rather than copied, so every figure a caller already
// reads stays where it was; the statistics are a group of their own because a spot
// run has none, and a figure that is always zero on one of the two kinds of run
// would read as a run that found nothing.
type KCandleContractHistorySyncRunDto struct {
	KCandleHistorySyncRunDto
	PositionStatistic ContractPositionStatisticSyncProgressDto `json:"positionStatistic"`
}

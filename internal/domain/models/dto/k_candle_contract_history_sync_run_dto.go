package dto

// KCandleContractHistorySyncRunDto embeds the spot run progress and adds position statistic
// progress, which spot runs lack.
type KCandleContractHistorySyncRunDto struct {
	KCandleHistorySyncRunDto
	PositionStatistic ContractPositionStatisticSyncProgressDto `json:"positionStatistic"`
}

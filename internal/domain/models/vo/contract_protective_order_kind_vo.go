package vo

// ContractProtectiveOrderKindVo is which side of an open position a protective order guards.
type ContractProtectiveOrderKindVo string

const (
	ContractProtectiveOrderStopLoss   ContractProtectiveOrderKindVo = "stopLoss"
	ContractProtectiveOrderTakeProfit ContractProtectiveOrderKindVo = "takeProfit"
)

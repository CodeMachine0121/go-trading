package vo

// ContractEntryStatusVo is how an auto order's entry order has gone; empty until one is placed.
type ContractEntryStatusVo string

const (
	ContractEntryWorking         ContractEntryStatusVo = "working"
	ContractEntryFilled          ContractEntryStatusVo = "filled"
	ContractEntryPartiallyFilled ContractEntryStatusVo = "partiallyFilled"
	ContractEntryUnfilled        ContractEntryStatusVo = "unfilled"
)

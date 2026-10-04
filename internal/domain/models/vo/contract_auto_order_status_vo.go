package vo

// ContractAutoOrderStatusVo is where an auto order is in its queue: waiting, being carried out by one replica, or finished.
type ContractAutoOrderStatusVo string

const (
	ContractAutoOrderReady     ContractAutoOrderStatusVo = "ready"
	ContractAutoOrderExecuting ContractAutoOrderStatusVo = "executing"
	ContractAutoOrderSettled   ContractAutoOrderStatusVo = "settled"
)

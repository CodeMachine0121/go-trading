package vo

// ContractAutoOrderOutcomeVo is how a settled auto order ended; an unsettled one is pending.
type ContractAutoOrderOutcomeVo string

const (
	ContractAutoOrderPending ContractAutoOrderOutcomeVo = "pending"
	// ContractAutoOrderFilled did everything it set out to do, possibly with its protection missing.
	ContractAutoOrderFilled ContractAutoOrderOutcomeVo = "filled"
	// ContractAutoOrderPartiallyDone closed the bot's position but could not open the new one.
	ContractAutoOrderPartiallyDone ContractAutoOrderOutcomeVo = "partiallyDone"
	ContractAutoOrderNotPlaced     ContractAutoOrderOutcomeVo = "notPlaced"
	// ContractAutoOrderAbandoned was given up before sending: too late, or the owner pulled the brake.
	ContractAutoOrderAbandoned ContractAutoOrderOutcomeVo = "abandoned"
)

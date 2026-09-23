package vo

// ContractOpeningOutcomeVo is what became of one attempt to open a contract position.
// Immutable, no behavior.
type ContractOpeningOutcomeVo string

const (
	// ContractOpeningOpened is a position that was opened.
	ContractOpeningOpened ContractOpeningOutcomeVo = "opened"
	// ContractOpeningUnaffordable is an opening the cash could not pay for, margin and
	// entry charge together. The replay carries on flat, exactly as spot does.
	ContractOpeningUnaffordable ContractOpeningOutcomeVo = "unaffordable"
	// ContractOpeningBlockedByTradingRules is an opening the venue would have refused:
	// too few units, too little notional, or more leverage than its tier allows. It is
	// counted, because a replay that quietly never trades reads like a cautious one.
	ContractOpeningBlockedByTradingRules ContractOpeningOutcomeVo = "blockedByTradingRules"
)

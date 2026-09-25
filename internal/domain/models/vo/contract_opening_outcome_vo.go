package vo

type ContractOpeningOutcomeVo string

const (
	ContractOpeningOpened ContractOpeningOutcomeVo = "opened"
	// ContractOpeningUnaffordable is an opening the cash could not cover (margin plus entry charge); the replay stays flat.
	ContractOpeningUnaffordable ContractOpeningOutcomeVo = "unaffordable"
	// ContractOpeningBlockedByTradingRules is an opening the venue would refuse; it is counted so a never-trading replay is not mistaken for a cautious one.
	ContractOpeningBlockedByTradingRules ContractOpeningOutcomeVo = "blockedByTradingRules"
)

package vo

// ContractOrderProgressVo is where an order stands at the venue: still working, fully filled, or ended without filling everything.
type ContractOrderProgressVo string

const (
	ContractOrderWorking ContractOrderProgressVo = "working"
	ContractOrderFilled  ContractOrderProgressVo = "filled"
	// ContractOrderEnded covers cancelled, expired and rejected; whatever it executed before ending stays executed.
	ContractOrderEnded ContractOrderProgressVo = "ended"
)

package vo

// TradeLedgerWordingVo is how a journal names its own entries and exits, so a ledger refusal reads in that journal's words.
type TradeLedgerWordingVo struct {
	Entry   string
	Exit    string
	Holding string
	Price   string
	// OverExitAdvice follows the refusal of an exit larger than what is held, such as how to reverse.
	OverExitAdvice  string
	ValidationError error
}

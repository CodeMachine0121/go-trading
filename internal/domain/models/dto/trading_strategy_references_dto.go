package dto

// TradingStrategyReferencesDto is the answer to "who is using this trading
// strategy" — both halves of it, in one shape.
//
// The three halves answer three different refusals and are read together every time:
// a rewrite is refused while any of the bots is running, a delete is refused while
// there is any bot at all, and a rewrite that takes away borrowing is refused while
// any bot is suggesting a loan. Asking separately would mean reads that can disagree
// with each other about the same moment.
//
// The two name lists carry names rather than counts because the refusals they produce
// have to tell somebody which bot to go and deal with; a number leaves them opening
// every bot they own to find out.
type TradingStrategyReferencesDto struct {
	TotalCount      int
	RunningBotNames []string
	// BorrowingBotNames is the bots whose position plan suggests more than the money
	// behind it. They are the ones a change of trading mode could strand: a bot
	// suggesting a loan against rules that cannot borrow is the state saving a bot
	// refuses, and it must not be reachable by changing the rules underneath it.
	BorrowingBotNames []string
}

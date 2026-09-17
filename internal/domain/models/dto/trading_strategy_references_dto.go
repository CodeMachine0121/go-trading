package dto

// TradingStrategyReferencesDto is the answer to "who is using this trading
// strategy" — both halves of it, in one shape.
//
// The two halves answer two different refusals and are read together every time:
// a rewrite is refused while any of the bots is running, and a delete is refused
// while there is any bot at all. Asking twice would mean two reads that can disagree
// with each other about the same moment.
//
// RunningBotNames carries names rather than a count because the refusal it produces
// has to tell somebody which bot to go and stop; a number leaves them opening every
// bot they own to find out.
type TradingStrategyReferencesDto struct {
	TotalCount      int
	RunningBotNames []string
}

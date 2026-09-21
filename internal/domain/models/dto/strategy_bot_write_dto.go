package dto

// StrategyBotWriteDto is a strategy bot as it arrives to be saved, before anything
// about it has been settled: the name still carries whatever blanks were typed
// around it, and the operators and signals are still just the spellings that came in.
//
// Creating and rewriting hand over the same shape, because every rule that applies
// to one applies word for word to the other. ID is zero on a create and names the
// bot on a rewrite; OwnerID is settled by whoever is signed in and is never read
// from the request.
type StrategyBotWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string
	Symbol  string
	// TradingStrategyID is the rules this bot is to follow. Exactly one, and it
	// has to be the caller's own — the rules are not given here, only named.
	TradingStrategyID      uint
	TriggerIntervalMinutes int
	// PositionPlan is what this bot is to suggest putting down each round, exactly as
	// declared. Whether it was filled in at all is read from the capital: leaving the
	// whole group empty is an ordinary thing to do, and such a bot suggests nothing.
	PositionPlan PositionPlanSettingsDto
	// TradingMode is how the rules this bot names trade — read from those rules, not
	// from the request, the same way OwnerID is read from whoever is signed in.
	//
	// It is here because one rule about a position plan is not a rule about the plan
	// at all: what a bot may suggest borrowing is limited by what its rules may
	// borrow, and the plan alone cannot answer that.
	TradingMode string
}

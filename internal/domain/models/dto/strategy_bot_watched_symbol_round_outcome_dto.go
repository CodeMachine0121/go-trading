package dto

import "github.com/shopspring/decimal"

// StrategyBotWatchedSymbolRoundOutcomeDto is what one round came to on one watched symbol; kinds never overlap:
// skipped has no halt reason or signal, halted and watchedSymbolHalted have only a reason, concluded has neither.
type StrategyBotWatchedSymbolRoundOutcomeDto struct {
	Symbol string
	// Kind is "skipped", "halted" (the whole bot), "watchedSymbolHalted" (this symbol alone) or "concluded".
	Kind       string
	HaltReason string
	// Verdict is buy, sell, none or conflict and is kept apart from SentSignal so history
	// records every round's view, not just what it queued to say.
	Verdict string
	// SentSignal is the signal the round queued a message for, empty when it queued none.
	SentSignal  string
	Conflicting bool
	// PositionPlan travels with the outcome so history keeps the figures this round used
	// even if settings change later.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
	// ReferencePrice and JournalLinkIdentifier travel only with a round that offered a journal link.
	ReferencePrice        decimal.NullDecimal
	JournalLinkIdentifier string
	// Round is what the round's message says, written when the round is booked in; only set when HasMessage.
	Round      StrategyBotRoundDto
	HasMessage bool
	// AutoOrderIntent is what a contract round would do with real money; whether it is queued is decided when the round is booked in.
	AutoOrderIntent    ContractAutoOrderIntentDto
	HasAutoOrderIntent bool
}

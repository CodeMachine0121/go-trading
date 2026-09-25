package dto

// StrategyBotRoundOutcomeDto kinds never overlap: skipped has no halt reason or signal,
// halted has only a reason, concluded has neither.
type StrategyBotRoundOutcomeDto struct {
	// Kind is "skipped", "halted" or "concluded".
	Kind       string
	HaltReason string
	// Verdict is buy, sell, none or conflict and is kept apart from SentSignal so history
	// records every round's view, not just what was sent.
	Verdict string
	// SentSignal is empty when nothing was sent.
	SentSignal  string
	Conflicting bool
	// PositionPlan travels with the outcome so history keeps the figures this round used
	// even if settings change later.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
}

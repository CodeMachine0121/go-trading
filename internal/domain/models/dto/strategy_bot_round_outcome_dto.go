package dto

// StrategyBotRoundOutcomeDto is what one round came to, in the shape the
// application layer passes it back for recording.
//
// It is a DTO rather than the domain model itself, because that is the only shape
// the application layer is allowed to hold. Reading it — deciding which of the three
// kinds this is, and what that does to a bot — stays in the domain.
//
// Its three kinds never overlap: a skipped round has no halt reason and no signal, a
// halted one has a reason and nothing else, and a concluded one has neither. Nothing
// here can say "skipped, and here is the signal it sent".
type StrategyBotRoundOutcomeDto struct {
	// Kind is "skipped", "halted" or "concluded". It is the one field that says how
	// to read the rest.
	Kind string
	// HaltReason is set only on a halted round.
	HaltReason string
	// SentSignal is what actually reached Telegram, on a concluded round. Empty
	// when nothing was sent — a conclusion nobody received has not been said.
	SentSignal string
	// Conflicting is whether the two conditions held at once, on a concluded round.
	Conflicting bool
}

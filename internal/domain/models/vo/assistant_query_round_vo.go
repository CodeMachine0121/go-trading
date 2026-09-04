package vo

// AssistantQueryRoundVo is one round trip that asked for lookups rather than
// answering: what the assistant said on the way, and every lookup it asked for
// together with what came back. Immutable plain data, no behavior.
//
// A round is the unit rather than a single lookup because that is the unit the
// assistant produced. Two things depend on keeping it whole:
//
// Its narration belongs to it. An assistant that says "let me check the existing
// scripts first" and asks for a lookup in the same breath must get that sentence
// back with the lookup, or its next turn continues from a thought it can no longer
// see — and it starts the answer mid-sentence.
//
// Its lookups belong together. When the assistant asks for three things at once,
// all three results have to come back as one reply; split into three, it learns
// that asking for several at once does not work and stops doing it — which costs
// a round trip per lookup from then on.
type AssistantQueryRoundVo struct {
	// Narration is what the assistant said alongside the requests, if it said
	// anything. It is not an answer: the answer comes after the lookups.
	Narration string
	Exchanges []AssistantQueryExchangeVo
}

package dto

// StrategyScriptWriteDto is the shape the application hands the domain to create or update
// a strategy script.
//
// The identifier is part of it, and that is what makes creating and updating share
// one set of rules rather than two that have to be kept in step by hand: zero names
// no strategy script yet, so it is a create; anything else names the strategy script being
// rewritten. Every rule is then written once and applies to both by construction.
type StrategyScriptWriteDto struct {
	ID uint
	// OwnerID is who this strategy script belongs to. On a create it is who it will belong
	// to; on a rewrite it is who is allowed to be doing the rewriting — the same
	// number answers both, because only an owner rewrites their own strategy script.
	OwnerID     uint
	Name        string
	Description string
	Script      string
	ResultType  string
	// MarketDataKind is the kind of market the algorithm eats, exactly as it was
	// written. Blank on a create means the spot K candle; blank on a rewrite means
	// the kind the strategy script already has.
	MarketDataKind string
	// Parameters are the algorithm's own knobs. Absent means an algorithm with no
	// knobs, which is what every algorithm written before knobs existed is.
	Parameters []StrategyScriptParameterWriteDto
}

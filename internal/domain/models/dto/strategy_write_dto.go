package dto

// StrategyWriteDto is the shape the application hands the domain to create or update
// a strategy.
//
// The identifier is part of it, and that is what makes creating and updating share
// one set of rules rather than two that have to be kept in step by hand: zero names
// no strategy yet, so it is a create; anything else names the strategy being
// rewritten. Every rule is then written once and applies to both by construction.
type StrategyWriteDto struct {
	ID                  uint
	Name                string
	Script              string
	ResultType          string
}

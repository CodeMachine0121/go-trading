package dto

// StrategyScriptWriteDto serves both create (zero ID) and rewrite so one set of rules covers both.
type StrategyScriptWriteDto struct {
	ID uint
	// OwnerID is the future owner on create and the required owner on rewrite.
	OwnerID     uint
	Name        string
	Description string
	Script      string
	ResultType  string
	// MarketDataKind blank means kCandle on create and unchanged on rewrite.
	MarketDataKind string
	// Parameters may be empty for scripts with no knobs.
	Parameters []StrategyScriptParameterWriteDto
}

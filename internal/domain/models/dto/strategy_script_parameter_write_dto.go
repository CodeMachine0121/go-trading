package dto

// StrategyScriptParameterWriteDto is unvalidated input: the name is untrimmed and the kind
// is the raw spelling.
type StrategyScriptParameterWriteDto struct {
	Name         string
	Kind         string
	DefaultValue float64
}

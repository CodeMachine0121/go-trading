package dto

// StrategyScriptParameterValueDto carries no kind because the script's declaration decides
// it; the JSON tags matter because bots round-trip these values.
type StrategyScriptParameterValueDto struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

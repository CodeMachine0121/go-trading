package dto

// StrategyScriptParameterDto is the only shape one strategy script parameter leaves the domain in.
type StrategyScriptParameterDto struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

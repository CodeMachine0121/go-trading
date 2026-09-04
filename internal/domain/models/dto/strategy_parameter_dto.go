package dto

// StrategyParameterDto is the only shape one strategy parameter leaves the domain in.
type StrategyParameterDto struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

package dto

// StrategyScriptParameterDto is the only shape one strategy script parameter leaves the domain in.
type StrategyScriptParameterDto struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// ToWriteDto hands a settled parameter back in the shape a parameter set is built from,
// so that a set can be rebuilt from what it once handed out — as a script compartment
// does with the values it was sent.
func (strategyScriptParameterDto StrategyScriptParameterDto) ToWriteDto() StrategyScriptParameterWriteDto {
	return StrategyScriptParameterWriteDto{
		Name:         strategyScriptParameterDto.Name,
		Kind:         strategyScriptParameterDto.Kind,
		DefaultValue: strategyScriptParameterDto.DefaultValue,
	}
}

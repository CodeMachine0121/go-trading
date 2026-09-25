package dto

type StrategyScriptParameterDto struct {
	Name         string  `json:"name"`
	Kind         string  `json:"kind"`
	DefaultValue float64 `json:"defaultValue"`
}

// ToWriteDto lets a parameter set be rebuilt from what it handed out.
func (strategyScriptParameterDto StrategyScriptParameterDto) ToWriteDto() StrategyScriptParameterWriteDto {
	return StrategyScriptParameterWriteDto{
		Name:         strategyScriptParameterDto.Name,
		Kind:         strategyScriptParameterDto.Kind,
		DefaultValue: strategyScriptParameterDto.DefaultValue,
	}
}

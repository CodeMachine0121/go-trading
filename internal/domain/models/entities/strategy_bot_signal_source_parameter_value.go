package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyBotSignalSourceParameterValue is what one of a strategy's knobs is worth
// inside one signal source.
//
// It is a row of its own rather than a blob on the source, for the same reason a
// strategy's declared parameters are: a knob is one name and one number, and a
// column that holds several of them can only be read by parsing it.
//
// The value is one float64 and the kind is absent, exactly as it is on a strategy
// parameter value: which kind a name was declared as is the strategy's word, not
// this bot's, so setting a value here cannot change what that name means.
type StrategyBotSignalSourceParameterValue struct {
	ID                        uint    `gorm:"primaryKey"`
	StrategyBotSignalSourceID uint    `gorm:"not null;index:idx_strategy_bot_source_parameter_values_source"`
	Name                      string  `gorm:"size:64;not null"`
	Value                     float64 `gorm:"not null"`
}

// TableName pins the table instead of using GORM's default.
func (parameterValue StrategyBotSignalSourceParameterValue) TableName() string {
	return "StrategyBotSignalSourceParameterValues"
}

// ToDto converts this row into the shape the domain hands outwards.
func (parameterValue StrategyBotSignalSourceParameterValue) ToDto() dto.StrategyParameterValueDto {
	return dto.StrategyParameterValueDto{
		Name:  parameterValue.Name,
		Value: parameterValue.Value,
	}
}

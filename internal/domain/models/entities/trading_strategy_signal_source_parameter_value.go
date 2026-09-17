package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingStrategySignalSourceParameterValue is what one of a strategy script's knobs is worth
// inside one signal source.
//
// It is a row of its own rather than a blob on the source, for the same reason a
// strategy script's declared parameters are: a knob is one name and one number, and a
// column that holds several of them can only be read by parsing it.
//
// The value is one float64 and the kind is absent, exactly as it is on a strategy script
// parameter value: which kind a name was declared as is the strategy script's word, not
// this bot's, so setting a value here cannot change what that name means.
type TradingStrategySignalSourceParameterValue struct {
	ID                            uint    `gorm:"primaryKey"`
	TradingStrategySignalSourceID uint    `gorm:"not null;index:idx_trading_strategy_source_parameter_values_source"`
	Name                          string  `gorm:"size:64;not null"`
	Value                         float64 `gorm:"not null"`
}

// TableName pins the table instead of using GORM's default.
func (parameterValue TradingStrategySignalSourceParameterValue) TableName() string {
	return "TradingStrategySignalSourceParameterValues"
}

// ToDto converts this row into the shape the domain hands outwards.
func (parameterValue TradingStrategySignalSourceParameterValue) ToDto() dto.StrategyScriptParameterValueDto {
	return dto.StrategyScriptParameterValueDto{
		Name:  parameterValue.Name,
		Value: parameterValue.Value,
	}
}

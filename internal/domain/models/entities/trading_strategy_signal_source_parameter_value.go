package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingStrategySignalSourceParameterValue is one parameter value inside one signal source; it has no kind because the kind is the strategy script's to declare.
type TradingStrategySignalSourceParameterValue struct {
	ID                            uint    `gorm:"primaryKey"`
	TradingStrategySignalSourceID uint    `gorm:"not null;index:idx_trading_strategy_source_parameter_values_source"`
	Name                          string  `gorm:"size:64;not null"`
	Value                         float64 `gorm:"not null"`
}

func (parameterValue TradingStrategySignalSourceParameterValue) TableName() string {
	return "TradingStrategySignalSourceParameterValues"
}

func (parameterValue TradingStrategySignalSourceParameterValue) ToDto() dto.StrategyScriptParameterValueDto {
	return dto.StrategyScriptParameterValueDto{
		Name:  parameterValue.Name,
		Value: parameterValue.Value,
	}
}

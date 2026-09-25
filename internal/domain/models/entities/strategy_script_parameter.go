package entities

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyScriptParameter is one tunable number of a strategy script; a single float64 suffices because look-back counts stay far below 2^53.
type StrategyScriptParameter struct {
	ID               uint   `gorm:"primaryKey"`
	StrategyScriptID uint   `gorm:"column:strategy_id;not null;index:idx_strategy_parameters_strategy"`
	Name             string `gorm:"size:64;not null"`
	// Kind is stored already normalized by the domain.
	Kind         string  `gorm:"size:32;not null"`
	DefaultValue float64 `gorm:"not null"`
}

// TableName keeps the pre-rename table name; see StrategyScript.TableName.
func (strategyScriptParameter StrategyScriptParameter) TableName() string {
	return "StrategyParameters"
}

func (strategyScriptParameter StrategyScriptParameter) ToDto() dto.StrategyScriptParameterDto {
	return dto.StrategyScriptParameterDto{
		Name:         strategyScriptParameter.Name,
		Kind:         strategyScriptParameter.Kind,
		DefaultValue: strategyScriptParameter.DefaultValue,
	}
}

// IsLookbackCount reports whether this parameter determines how many candles to read.
func (strategyScriptParameter StrategyScriptParameter) IsLookbackCount() bool {
	return vo.StrategyScriptParameterKindVo(strategyScriptParameter.Kind) == vo.StrategyScriptParameterKindLookbackCount
}

func (strategyScriptParameter StrategyScriptParameter) IsBoolean() bool {
	return vo.StrategyScriptParameterKindVo(strategyScriptParameter.Kind) == vo.StrategyScriptParameterKindBoolean
}

// IsTrue reads the parameter as a boolean: zero is false, anything else is true.
func (strategyScriptParameter StrategyScriptParameter) IsTrue() bool {
	return strategyScriptParameter.DefaultValue != 0
}

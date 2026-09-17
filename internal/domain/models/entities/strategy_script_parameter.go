package entities

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyScriptParameter is one knob a strategy script carries: a number a person can change
// when running it, without editing the algorithm.
//
// The value is one float64 and the kind says how to read it, rather than two columns
// of which one is always meaningless — because there are not two types here. A
// person types one number into one box; what differs is what the system does with
// it. A look-back count kept in a float64 is exact: whole numbers are exact in a
// float64 up to 2^53, and a look-back is capped by the single-query limit, which is
// smaller by a factor of ten trillion.
//
// It is a plain data model: fields, persistence mapping and shape conversion only.
type StrategyScriptParameter struct {
	ID               uint   `gorm:"primaryKey"`
	StrategyScriptID uint   `gorm:"column:strategy_id;not null;index:idx_strategy_parameters_strategy"`
	Name             string `gorm:"size:64;not null"`
	// Kind is stored as it was settled, never as it was typed: the domain normalizes
	// and refuses before anything reaches here.
	Kind         string  `gorm:"size:32;not null"`
	DefaultValue float64 `gorm:"not null"`
}

// TableName pins the table to StrategyParameters, the name it had before this
// model was renamed. See StrategyScript.TableName for why the old name stays.
func (strategyScriptParameter StrategyScriptParameter) TableName() string {
	return "StrategyParameters"
}

// ToDto converts this record into the shape the domain hands outwards.
func (strategyScriptParameter StrategyScriptParameter) ToDto() dto.StrategyScriptParameterDto {
	return dto.StrategyScriptParameterDto{
		Name:         strategyScriptParameter.Name,
		Kind:         strategyScriptParameter.Kind,
		DefaultValue: strategyScriptParameter.DefaultValue,
	}
}

// IsLookbackCount says whether this knob is one the system reads meaning into — how
// many candles to read is derived from these, and nothing else about a kind changes
// what the system does.
func (strategyScriptParameter StrategyScriptParameter) IsLookbackCount() bool {
	return vo.StrategyScriptParameterKindVo(strategyScriptParameter.Kind) == vo.StrategyScriptParameterKindLookbackCount
}

// IsBoolean says whether this knob is a yes-or-no. Only two things ask: settling the
// value to exactly zero or one, and handing it to a script as a bool.
func (strategyScriptParameter StrategyScriptParameter) IsBoolean() bool {
	return vo.StrategyScriptParameterKindVo(strategyScriptParameter.Kind) == vo.StrategyScriptParameterKindBoolean
}

// IsTrue reads this knob as a yes-or-no: zero is no, anything else is yes.
func (strategyScriptParameter StrategyScriptParameter) IsTrue() bool {
	return strategyScriptParameter.DefaultValue != 0
}

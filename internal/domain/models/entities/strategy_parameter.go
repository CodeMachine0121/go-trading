package entities

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyParameter is one knob a strategy carries: a number a person can change
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
type StrategyParameter struct {
	ID         uint   `gorm:"primaryKey"`
	StrategyID uint   `gorm:"not null;index:idx_strategy_parameters_strategy"`
	Name       string `gorm:"size:64;not null"`
	// Kind is stored as it was settled, never as it was typed: the domain normalizes
	// and refuses before anything reaches here.
	Kind         string  `gorm:"size:32;not null"`
	DefaultValue float64 `gorm:"not null"`
}

// TableName pins the table to StrategyParameters instead of GORM's default.
func (strategyParameter StrategyParameter) TableName() string {
	return "StrategyParameters"
}

// ToDto converts this record into the shape the domain hands outwards.
func (strategyParameter StrategyParameter) ToDto() dto.StrategyParameterDto {
	return dto.StrategyParameterDto{
		Name:         strategyParameter.Name,
		Kind:         strategyParameter.Kind,
		DefaultValue: strategyParameter.DefaultValue,
	}
}

// IsLookbackCount says whether this knob is one the system reads meaning into. It is
// the only question anything asks about a kind, which is why nothing branches per
// kind anywhere else.
func (strategyParameter StrategyParameter) IsLookbackCount() bool {
	return vo.StrategyParameterKindVo(strategyParameter.Kind) == vo.StrategyParameterKindLookbackCount
}

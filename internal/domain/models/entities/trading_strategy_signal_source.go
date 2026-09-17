package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingStrategySignalSource is one strategy script as it runs inside one trading
// strategy.
//
// The label carries a unique index spanning the trading strategy, because a label is
// how a condition names a source and two sources answering to "A" would make a
// condition mean two things. The index — not a read-then-write check — is what makes it true.
//
// There is no copy of the strategy script's name. A copy would be wrong the first time
// somebody renamed the strategy script, and nothing needs it: the label is already this
// source's name, chosen by the person who reads the conditions.
type TradingStrategySignalSource struct {
	ID                uint   `gorm:"primaryKey"`
	TradingStrategyID uint   `gorm:"not null;index:idx_trading_strategy_signal_sources_strategy;uniqueIndex:idx_trading_strategy_signal_sources_strategy_label"`
	Label             string `gorm:"size:32;not null;uniqueIndex:idx_trading_strategy_signal_sources_strategy_label"`
	// StrategyScriptID is not a declared association, and that is deliberate. A cascade
	// from the strategy script would make deleting a strategy script silently gut every
	// trading strategy that used it; the bots using it are supposed to halt with a
	// reason their owner can read instead.
	StrategyScriptID    uint   `gorm:"column:strategy_id;not null;index:idx_trading_strategy_signal_sources_script"`
	AggregationInterval string `gorm:"size:16;not null"`

	ParameterValues []TradingStrategySignalSourceParameterValue `gorm:"foreignKey:TradingStrategySignalSourceID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table instead of using GORM's default.
func (tradingStrategySignalSource TradingStrategySignalSource) TableName() string {
	return "TradingStrategySignalSources"
}

// ToDto converts this row into the shape the domain hands outwards. There is no
// line dropping the script, because the shape has nowhere to put one.
func (tradingStrategySignalSource TradingStrategySignalSource) ToDto() dto.TradingStrategySignalSourceDto {
	return dto.TradingStrategySignalSourceDto{
		Label:               tradingStrategySignalSource.Label,
		StrategyScriptID:    tradingStrategySignalSource.StrategyScriptID,
		AggregationInterval: tradingStrategySignalSource.AggregationInterval,
		ParameterValues:     tradingStrategySignalSource.parameterValueDtos(),
	}
}

// parameterValueDtos hands out this source's values, always as a list rather than
// sometimes nothing: a source with no values set has an empty list, not an absence.
func (tradingStrategySignalSource TradingStrategySignalSource) parameterValueDtos() []dto.StrategyScriptParameterValueDto {
	parameterValueDtos := make(
		[]dto.StrategyScriptParameterValueDto, 0, len(tradingStrategySignalSource.ParameterValues))
	for _, parameterValue := range tradingStrategySignalSource.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, parameterValue.ToDto())
	}

	return parameterValueDtos
}

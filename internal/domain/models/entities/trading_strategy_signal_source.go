package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingStrategySignalSource is one strategy script running inside one trading strategy; the label is unique per trading strategy because conditions refer to sources by it.
type TradingStrategySignalSource struct {
	ID                uint   `gorm:"primaryKey"`
	TradingStrategyID uint   `gorm:"not null;index:idx_trading_strategy_signal_sources_strategy;uniqueIndex:idx_trading_strategy_signal_sources_strategy_label"`
	Label             string `gorm:"size:32;not null;uniqueIndex:idx_trading_strategy_signal_sources_strategy_label"`
	// StrategyScriptID deliberately has no cascade: deleting a script should halt the bots using it, not silently gut their strategies.
	StrategyScriptID    uint   `gorm:"column:strategy_id;not null;index:idx_trading_strategy_signal_sources_script"`
	AggregationInterval string `gorm:"size:16;not null"`

	ParameterValues []TradingStrategySignalSourceParameterValue `gorm:"foreignKey:TradingStrategySignalSourceID;constraint:OnDelete:CASCADE"`
}

func (tradingStrategySignalSource TradingStrategySignalSource) TableName() string {
	return "TradingStrategySignalSources"
}

func (tradingStrategySignalSource TradingStrategySignalSource) ToDto() dto.TradingStrategySignalSourceDto {
	return dto.TradingStrategySignalSourceDto{
		Label:               tradingStrategySignalSource.Label,
		StrategyScriptID:    tradingStrategySignalSource.StrategyScriptID,
		AggregationInterval: tradingStrategySignalSource.AggregationInterval,
		ParameterValues:     tradingStrategySignalSource.parameterValueDtos(),
	}
}

// parameterValueDtos always returns a non-nil list.
func (tradingStrategySignalSource TradingStrategySignalSource) parameterValueDtos() []dto.StrategyScriptParameterValueDto {
	parameterValueDtos := make(
		[]dto.StrategyScriptParameterValueDto, 0, len(tradingStrategySignalSource.ParameterValues))
	for _, parameterValue := range tradingStrategySignalSource.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, parameterValue.ToDto())
	}

	return parameterValueDtos
}

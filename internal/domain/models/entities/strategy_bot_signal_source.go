package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// StrategyBotSignalSource is one strategy as it runs inside one bot.
//
// The label carries a unique index spanning the bot, because a label is how a
// condition names a source and two sources answering to "A" would make a condition
// mean two things. The index — not a read-then-write check — is what makes it true.
//
// There is no copy of the strategy's name. A copy would be wrong the first time
// somebody renamed the strategy, and nothing needs it: the label is already this
// source's name, chosen by the person who reads the conditions.
type StrategyBotSignalSource struct {
	ID            uint   `gorm:"primaryKey"`
	StrategyBotID uint   `gorm:"not null;index:idx_strategy_bot_signal_sources_bot;uniqueIndex:idx_strategy_bot_signal_sources_bot_label"`
	Label         string `gorm:"size:32;not null;uniqueIndex:idx_strategy_bot_signal_sources_bot_label"`
	// StrategyID is not a declared association, and that is deliberate. A cascade
	// from the strategy would make deleting a strategy silently gut every bot that
	// used it; the bot is supposed to halt with a reason its owner can read instead.
	StrategyID          uint   `gorm:"not null;index:idx_strategy_bot_signal_sources_strategy"`
	AggregationInterval string `gorm:"size:16;not null"`

	ParameterValues []StrategyBotSignalSourceParameterValue `gorm:"foreignKey:StrategyBotSignalSourceID;constraint:OnDelete:CASCADE"`
}

// TableName pins the table instead of using GORM's default.
func (strategyBotSignalSource StrategyBotSignalSource) TableName() string {
	return "StrategyBotSignalSources"
}

// ToDto converts this row into the shape the domain hands outwards. There is no
// line dropping the script, because the shape has nowhere to put one.
func (strategyBotSignalSource StrategyBotSignalSource) ToDto() dto.StrategyBotSignalSourceDto {
	return dto.StrategyBotSignalSourceDto{
		Label:               strategyBotSignalSource.Label,
		StrategyID:          strategyBotSignalSource.StrategyID,
		AggregationInterval: strategyBotSignalSource.AggregationInterval,
		ParameterValues:     strategyBotSignalSource.parameterValueDtos(),
	}
}

// parameterValueDtos hands out this source's values, always as a list rather than
// sometimes nothing: a source with no values set has an empty list, not an absence.
func (strategyBotSignalSource StrategyBotSignalSource) parameterValueDtos() []dto.StrategyParameterValueDto {
	parameterValueDtos := make(
		[]dto.StrategyParameterValueDto, 0, len(strategyBotSignalSource.ParameterValues))
	for _, parameterValue := range strategyBotSignalSource.ParameterValues {
		parameterValueDtos = append(parameterValueDtos, parameterValue.ToDto())
	}

	return parameterValueDtos
}

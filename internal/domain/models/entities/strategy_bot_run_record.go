package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// StrategyBotRunRecord is one round a bot ran; the suggested figures are stored rather than recomputed because the bot's settings may change later.
type StrategyBotRunRecord struct {
	ID            uint `gorm:"primaryKey"`
	StrategyBotID uint `gorm:"not null;index:idx_strategy_bot_run_records_bot_number,priority:1"`
	// RunNumber keeps climbing after old rounds are trimmed, so a round's number never changes.
	RunNumber int       `gorm:"not null;index:idx_strategy_bot_run_records_bot_number,priority:2,sort:desc"`
	RanAt     time.Time `gorm:"type:timestamptz;not null"`
	// Suggested* figures are nullable because zero is a legitimate stop-loss price and must differ from "no suggestion".
	SuggestedStake           decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedStopLossPrice   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedTakeProfitPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// SuggestedDirection, SuggestedLeverage and SuggestedNotional are filled only for contract rounds with a suggestion.
	SuggestedDirection string              `gorm:"size:8;not null;default:''"`
	SuggestedLeverage  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedNotional  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// Result is buy, sell, hold or conflict — see StrategyBotRoundResultVo.
	Result string `gorm:"size:16;not null"`
}

func (strategyBotRunRecord StrategyBotRunRecord) TableName() string {
	return "StrategyBotRunRecords"
}

func (strategyBotRunRecord StrategyBotRunRecord) ToDto() dto.StrategyBotRunRecordDto {
	return dto.StrategyBotRunRecordDto{
		RunNumber:                strategyBotRunRecord.RunNumber,
		RanAt:                    strategyBotRunRecord.RanAt.UTC(),
		Result:                   strategyBotRunRecord.Result,
		SuggestedStake:           figureOrNothing(strategyBotRunRecord.SuggestedStake),
		SuggestedStopLossPrice:   figureOrNothing(strategyBotRunRecord.SuggestedStopLossPrice),
		SuggestedTakeProfitPrice: figureOrNothing(strategyBotRunRecord.SuggestedTakeProfitPrice),
		SuggestedDirection:       strategyBotRunRecord.SuggestedDirection,
		SuggestedLeverage:        figureOrNothing(strategyBotRunRecord.SuggestedLeverage),
		SuggestedNotional:        figureOrNothing(strategyBotRunRecord.SuggestedNotional),
	}
}

// figureOrNothing returns nil for an absent figure so it is not confused with a real zero.
func figureOrNothing(storedFigure decimal.NullDecimal) *decimal.Decimal {
	if !storedFigure.Valid {
		return nil
	}

	figure := storedFigure.Decimal

	return &figure
}

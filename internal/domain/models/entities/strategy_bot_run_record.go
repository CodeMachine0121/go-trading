package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// StrategyBotRunRecord is what one round concluded on one watched symbol; the suggested figures are stored rather than recomputed because the bot's settings may change later.
type StrategyBotRunRecord struct {
	ID            uint `gorm:"primaryKey"`
	StrategyBotID uint `gorm:"not null;index:idx_strategy_bot_run_records_bot_number,priority:1;index:idx_strategy_bot_run_records_bot_symbol_number,priority:1"`
	// Symbol is the watched symbol this record speaks for; records predating watched symbols were given their bot's only symbol.
	Symbol string `gorm:"size:64;not null;default:'';index:idx_strategy_bot_run_records_bot_symbol_number,priority:2"`
	// RunNumber is unique across all of a bot's symbols and keeps climbing after old rounds are trimmed, so a record's number never changes and names its symbol on its own.
	RunNumber int       `gorm:"not null;index:idx_strategy_bot_run_records_bot_number,priority:2,sort:desc;index:idx_strategy_bot_run_records_bot_symbol_number,priority:3,sort:desc"`
	RanAt     time.Time `gorm:"type:timestamptz;not null"`
	// Suggested* figures are nullable because zero is a legitimate stop-loss price and must differ from "no suggestion".
	SuggestedStake           decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedStopLossPrice   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedTakeProfitPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// SuggestedDirection, SuggestedLeverage and SuggestedNotional are filled only for contract rounds with a suggestion.
	SuggestedDirection string              `gorm:"size:8;not null;default:''"`
	SuggestedLeverage  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedNotional  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// ReferencePrice, SuggestedQuantity and JournalLinkIdentifier let the journal link prefill a trade from this very round.
	ReferencePrice        decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedQuantity     decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	JournalLinkIdentifier string              `gorm:"size:64;not null;default:'';index"`
	// Result is buy, sell, hold or conflict — see StrategyBotRoundResultVo.
	Result string `gorm:"size:16;not null"`
}

func (strategyBotRunRecord StrategyBotRunRecord) TableName() string {
	return "StrategyBotRunRecords"
}

func (strategyBotRunRecord StrategyBotRunRecord) ToDto() dto.StrategyBotRunRecordDto {
	return dto.StrategyBotRunRecordDto{
		RunNumber:                strategyBotRunRecord.RunNumber,
		Symbol:                   strategyBotRunRecord.Symbol,
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

// ToJournalLinkRoundDto pairs the round with the bot that ran it, which is all a journal link prefills from.
func (strategyBotRunRecord StrategyBotRunRecord) ToJournalLinkRoundDto(strategyBot StrategyBot) dto.JournalLinkRoundDto {
	return dto.JournalLinkRoundDto{
		StrategyBotID:            strategyBot.ID,
		StrategyBotName:          strategyBot.Name,
		Symbol:                   strategyBotRunRecord.Symbol,
		MarketDataKind:           strategyBot.MarketDataKind,
		TradingStrategyID:        strategyBot.TradingStrategyID,
		RunNumber:                strategyBotRunRecord.RunNumber,
		RanAt:                    strategyBotRunRecord.RanAt.UTC(),
		Result:                   strategyBotRunRecord.Result,
		ReferencePrice:           strategyBotRunRecord.ReferencePrice,
		SuggestedStake:           strategyBotRunRecord.SuggestedStake,
		SuggestedQuantity:        strategyBotRunRecord.SuggestedQuantity,
		SuggestedDirection:       strategyBotRunRecord.SuggestedDirection,
		SuggestedLeverage:        strategyBotRunRecord.SuggestedLeverage,
		SuggestedStopLossPrice:   strategyBotRunRecord.SuggestedStopLossPrice,
		SuggestedTakeProfitPrice: strategyBotRunRecord.SuggestedTakeProfitPrice,
	}
}

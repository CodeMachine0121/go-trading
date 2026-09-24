package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// StrategyBotRunRecord is one round a bot ran: when, and what it came to.
//
// It is a plain data model: fields, persistence mapping and shape conversion only.
//
// What it keeps is deliberately narrow. A round's full working — which source said
// what, which condition held — is a great deal of writing for a question nobody has
// asked; and a history nobody reads is a table that only grows.
//
// The three figures a suggestion came to are the exception, and they earned it: a
// message told somebody to put down five thousand and stop at 66105.915, and by the
// time they read that round back their own settings may well have changed. Recomputing
// then would answer about today rather than about that round, so the figures are
// remembered rather than derived.
type StrategyBotRunRecord struct {
	ID uint `gorm:"primaryKey"`
	// StrategyBotID is what the cascade hangs off — the bot declares the
	// association, so deleting one takes its rounds with it. A history belonging to
	// a bot nobody can reach is rows that stay for ever.
	StrategyBotID uint `gorm:"not null;index:idx_strategy_bot_run_records_bot_number,priority:1"`
	// RunNumber is what this round is called: Run 1, Run 2, and so on for this bot.
	//
	// It keeps climbing as old rounds are trimmed away, so Run 51 stays Run 51 even
	// once Run 1 is gone. Renumbering would mean the same round answered to two
	// different names depending on when somebody looked.
	//
	// It is indexed with the bot and descending, because the only read is "this
	// bot's latest few, newest first".
	RunNumber int `gorm:"not null;index:idx_strategy_bot_run_records_bot_number,priority:2,sort:desc"`
	// RanAt is the moment the round finished. Stored in universal time, like every
	// other moment here.
	RanAt time.Time `gorm:"type:timestamptz;not null"`
	// SuggestedStake, SuggestedStopLossPrice and SuggestedTakeProfitPrice are what
	// this round suggested putting down and where it suggested getting out.
	//
	// All three may be absent, and absent is the ordinary case: a bot with no
	// position plan suggests nothing, and a round that concluded nothing to open
	// suggests nothing either. They are nullable rather than zero-defaulted because a
	// stop-loss price of zero is a legitimate figure — a distance of the whole price —
	// so nothing would be indistinguishable from that.
	SuggestedStake           decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedStopLossPrice   decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedTakeProfitPrice decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// SuggestedDirection, SuggestedLeverage and SuggestedNotional are which way a
	// contract round's suggestion faced, how many times its margin it carried, and what
	// that came to. Only a contract round with a suggestion fills them in: a spot one
	// only ever faces long at one times, and saying so on every row is noise that reads
	// like information.
	SuggestedDirection string              `gorm:"size:8;not null;default:''"`
	SuggestedLeverage  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	SuggestedNotional  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	// Result is buy, sell, hold or conflict — see StrategyBotRoundResultVo.
	//
	// Conflicted has a word of its own because it is the one quiet round that asks
	// for something: that bot will stay silent until its owner changes a condition.
	// Every other way a round ends without a position is still "hold"; why it ended
	// that way is on the bot itself, where it can be acted on.
	Result string `gorm:"size:16;not null"`
}

// TableName pins the table instead of using GORM's default.
func (strategyBotRunRecord StrategyBotRunRecord) TableName() string {
	return "StrategyBotRunRecords"
}

// ToDto converts this row into the shape the domain hands outwards. The moment is
// always handed out in universal time, whatever zone it was read back in.
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

// figureOrNothing is one stored figure as the answer carries it: the number when the
// round suggested one, and nothing at all when it did not.
//
// The two are told apart rather than collapsed, because zero is a figure a stop-loss
// price really can be — a distance of the whole price — and a history that answered
// zero for both would have somebody reading a stop at zero on every quiet round.
func figureOrNothing(storedFigure decimal.NullDecimal) *decimal.Decimal {
	if !storedFigure.Valid {
		return nil
	}

	figure := storedFigure.Decimal

	return &figure
}

package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// StrategyBotRunRecord is one round a bot ran: when, and what it came to.
//
// It is a plain data model: fields, persistence mapping and shape conversion only.
//
// Only three things are kept. A round's full working — which source said what,
// which condition held — is a great deal of writing for a question nobody has asked
// yet; and a history nobody reads is a table that only grows. What people actually
// ask a history is "did it run, and what did it say", which is exactly this.
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
		RunNumber: strategyBotRunRecord.RunNumber,
		RanAt:     strategyBotRunRecord.RanAt.UTC(),
		Result:    strategyBotRunRecord.Result,
	}
}

package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// StrategyBot is one standing bot: which trading strategy it follows, which market
// it watches, and how often it asks.
//
// The rules themselves live in the trading strategy it points at, not here. That is
// what lets three bots watch three markets by the same rules instead of each
// carrying a copy that drifts from the other two.
//
// It is a plain data model: fields, persistence mapping and shape conversion only,
// no business rules.
//
// The name carries a unique index spanning the owner, for the reason a strategy script's
// does: a name is what its owner recognises a bot by, nobody recognises a
// stranger's, and an index — not a read-then-write check — is what actually makes it
// unique.
//
// RunState, NextRunAt, LastSentSignal, HaltReason and Conflicting are stored rather
// than held in memory. That is the difference between a bot that survives a restart
// and one that quietly stops the next time the process comes back up, with its owner
// still believing it is watching.
type StrategyBot struct {
	ID uint `gorm:"primaryKey"`
	// OwnerID is settled when the bot is created and never changes: it is not on
	// the list of columns a rewrite may touch, so "a bot cannot change hands" is
	// something the write path cannot express rather than something it remembers
	// not to do.
	OwnerID uint   `gorm:"not null;index:idx_strategy_bots_owner;uniqueIndex:idx_strategy_bots_owner_name"`
	Name    string `gorm:"size:128;not null;uniqueIndex:idx_strategy_bots_owner_name"`
	Symbol  string `gorm:"size:64;not null"`
	// TradingStrategyID is the rules this bot follows. It defaults to zero so that
	// the column can be added to a table that already has rows in it; zero is the
	// mark the one-time move reads and clears, and no bot carries it afterwards.
	//
	// There is deliberately no foreign key behind it. One would have to hold the
	// moment the column is added — while every existing bot still carries the zero
	// the move has not yet cleared — so the database would refuse the very upgrade
	// that fills it in. What a foreign key would have bought is bought instead by
	// refusing to delete a trading strategy any bot still follows, a layer up,
	// where the refusal can say how many bots that is.
	TradingStrategyID uint `gorm:"not null;default:0;index:idx_strategy_bots_trading_strategy"`
	// TriggerIntervalMinutes is counted in minutes because that is the unit a
	// person thinks in here, and because the finest candle is one minute — asking
	// more often than that only fetches the same candle again.
	TriggerIntervalMinutes int `gorm:"not null"`
	// PositionPlanCapital is the money this bot sizes a suggested position against,
	// and it is the switch for the five columns below it: without money there is
	// nothing to stake, so a zero here means this bot has no position plan at all.
	//
	// All six default to zero, so every bot stored before they existed reads as
	// having no plan — and sends the message it sent before plans existed.
	//
	// They are exact decimals rather than floats, including the leverage and the two
	// distances, because all three multiply into money. A float would start drifting
	// a stop price around its tenth digit, and that price is one somebody places an
	// order at.
	PositionPlanCapital decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	// PositionPlanSizingMode is how much of that capital one opening stakes, in the
	// replay's own three spellings, and PositionPlanSizingValue the figure beside it.
	PositionPlanSizingMode  string          `gorm:"size:16;not null;default:''"`
	PositionPlanSizingValue decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	// PositionPlanStopLossPercentage and PositionPlanTakeProfitPercentage are how far
	// from the reference price each exit sits. Either may be left out on its own.
	PositionPlanStopLossPercentage   decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	PositionPlanTakeProfitPercentage decimal.Decimal `gorm:"type:numeric(38,18);not null;default:0"`
	// RunState is indexed together with NextRunAt because the scan asks exactly one
	// question of this table — which bots are running and due — and that pair is it.
	RunState string `gorm:"size:16;not null;index:idx_strategy_bots_run_state_next_run_at,priority:1"`
	// NextRunAt is when this bot is next entitled to a round. Starting sets it to
	// now, so pressing play acts immediately instead of after a whole interval; a
	// finished round sets it to now plus the interval, which is also why missed
	// rounds are never made up — the next one is measured from the round that
	// actually happened, not from the one that should have.
	NextRunAt time.Time `gorm:"type:timestamptz;index:idx_strategy_bots_run_state_next_run_at,priority:2"`
	// LastSentSignal is the last signal that reached Telegram. Cleared on start, so
	// the first conclusion after pressing play always goes out: somebody who
	// pressed play and heard nothing all night cannot tell a quiet market from a
	// broken bot.
	LastSentSignal string `gorm:"size:16;not null;default:''"`
	// HaltReason is empty unless the system stopped this bot itself.
	HaltReason string `gorm:"size:32;not null;default:''"`
	// Conflicting records that the last round found both conditions holding. It is
	// not a halt and it clears itself on the next round that does not conflict.
	Conflicting bool      `gorm:"not null;default:false"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null"`

	Owner User `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	// TradingStrategy is declared so that a list of bots can say which rules each
	// one follows without a second read per bot, and so that a bot never keeps a
	// stale copy of a name somebody has since changed.
	//
	// It is declared without a constraint, so that reading through it adds no
	// foreign key — see TradingStrategyID for why there cannot be one.
	TradingStrategy TradingStrategy `gorm:"foreignKey:TradingStrategyID;references:ID;constraint:-"`
	// RunRecords is declared for the cascade and for nothing else: a history is read
	// on its own, through its own repository, and never travels with the bot. What
	// it buys here is that deleting a bot takes its rounds with it — otherwise they
	// stay in the table for ever, belonging to something nobody can reach.
	RunRecords []StrategyBotRunRecord `gorm:"foreignKey:StrategyBotID;constraint:OnDelete:CASCADE"`
}

// PositionPlanSettingsDto is this bot's five position plan settings, in the shape the
// domain hands outwards. It is on the row because every field it reads is the row's
// own.
//
// Exported because a second reader arrived: answering "is this bot suggesting a loan"
// means building the plan from these five, and that question is asked from outside
// this package. A shape conversion reading only the row's own fields is not business
// logic, so it stays here rather than moving.
func (strategyBot StrategyBot) PositionPlanSettingsDto() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              strategyBot.PositionPlanCapital,
		SizingMode:           strategyBot.PositionPlanSizingMode,
		SizingValue:          strategyBot.PositionPlanSizingValue,
		StopLossPercentage:   strategyBot.PositionPlanStopLossPercentage,
		TakeProfitPercentage: strategyBot.PositionPlanTakeProfitPercentage,
	}
}

// TableName pins the table to StrategyBots instead of GORM's default.
func (strategyBot StrategyBot) TableName() string {
	return "StrategyBots"
}

// ToDto converts this record into the shape the domain hands outwards.
//
// It carries the trading strategy's name as well as its identifier, because a list
// of bots is read to see what each one is doing, and an identifier on its own says
// nothing. The name is read through the association every time rather than copied
// onto the bot, so renaming a trading strategy cannot leave a bot saying the old one.
func (strategyBot StrategyBot) ToDto() dto.StrategyBotDto {
	return dto.StrategyBotDto{
		ID:                     strategyBot.ID,
		OwnerID:                strategyBot.OwnerID,
		Name:                   strategyBot.Name,
		Symbol:                 strategyBot.Symbol,
		TriggerIntervalMinutes: strategyBot.TriggerIntervalMinutes,
		PositionPlan:           strategyBot.PositionPlanSettingsDto(),
		NextRunAt:              strategyBot.NextRunAt.UTC(),
		TradingStrategyID:      strategyBot.TradingStrategyID,
		TradingStrategyName:    strategyBot.TradingStrategy.Name,
		RunState:               strategyBot.RunState,
		LastSentSignal:         strategyBot.LastSentSignal,
		HaltReason:             strategyBot.HaltReason,
		Conflicting:            strategyBot.Conflicting,
		CreatedAt:              strategyBot.CreatedAt.UTC(),
		UpdatedAt:              strategyBot.UpdatedAt.UTC(),
	}
}

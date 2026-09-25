package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// StrategyBotRoundDto carries each source's signal, not just the verdict, so the message
// reader can judge it.
type StrategyBotRoundDto struct {
	BotName string
	Symbol  string
	// ContractTradingMode is blank on spot bots; with MarketDataKind it decides what action
	// the message asks for.
	MarketDataKind      string
	ContractTradingMode string
	// Verdict is buy or sell; rounds with nothing to say never reach a message.
	Verdict string
	// ReferencePrice is the latest finished one-minute close, deliberately not a fill price
	// since nothing is traded and sources may use different intervals.
	ReferencePrice       decimal.Decimal
	ReferenceTime        time.Time
	HasReference         bool
	SourceSignals        []StrategyBotSourceSignalDto
	PositionPlanSettings PositionPlanSettingsDto
	// PositionPlan is computed once by the domain and shared by the message and history so
	// they cannot disagree.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
}

type StrategyBotSourceSignalDto struct {
	Label               string
	AggregationInterval string
	Signal              string
}

// StrategyBotRoundDecisionDto decides whether to send here because that depends on the bot's
// last sent signal.
type StrategyBotRoundDecisionDto struct {
	Verdict     string
	ShouldSend  bool
	Conflicting bool
}

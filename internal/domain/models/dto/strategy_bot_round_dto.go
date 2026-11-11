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
	// AllocationPercentage is the watched symbol's share of the plan's capital; zero leaves the capital whole.
	AllocationPercentage decimal.Decimal
	// PositionPlan is computed once by the domain and shared by the message and history so
	// they cannot disagree.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
	// JournalLinkUrl is blank unless the round suggests opening a contract position.
	JournalLinkIdentifier string
	JournalLinkUrl        string
	// ReturnsToHold makes this the one-off message saying the symbol is back to holding: no position plan, no journal link.
	ReturnsToHold bool
	// RunNumber is the round's place in its bot's history, known only once the round is booked in; zero prints no number.
	RunNumber int
}

type StrategyBotSourceSignalDto struct {
	Label               string
	AggregationInterval string
	Signal              string
}

// StrategyBotRoundDecisionDto decides whether to send here because that depends on the bot's
// last sent signal.
type StrategyBotRoundDecisionDto struct {
	Verdict string
	// ShouldSend means the round decided a new signal, whether or not its message is sent.
	ShouldSend  bool
	Conflicting bool
	// Opinion is buy, sell or hold, blank for a conflict; the watched symbol keeps it to tell a return to holding.
	Opinion string
	// SpeaksSignal sends the decided signal's message; SpeaksReturnToHold sends the one-off return-to-hold message.
	SpeaksSignal       bool
	SpeaksReturnToHold bool
}

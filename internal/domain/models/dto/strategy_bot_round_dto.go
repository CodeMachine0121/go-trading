package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// StrategyBotRoundDto is everything one round produced, in the shape a message is
// written from.
//
// It carries what each source said, not just the conclusion. A person reading
// "sell" on their phone with no way to see why has only the option of believing it;
// the per-source lines are what make the message something they can judge.
type StrategyBotRoundDto struct {
	BotName string
	Symbol  string
	// Verdict is this round's conclusion: buy or sell. A round with nothing to say
	// never reaches a message.
	Verdict string
	// ReferencePrice is the close of this symbol's latest finished one-minute
	// candle, and HasReference says whether there was one to read. It is deliberately
	// not a fill price: nothing here buys anything, and a message that called it an
	// entry price would have somebody believing a trade was placed.
	//
	// One minute rather than "the candle the verdict rests on", because the sources
	// may run at different coarseness and there is no single such candle.
	ReferencePrice decimal.Decimal
	ReferenceTime  time.Time
	HasReference   bool
	SourceSignals  []StrategyBotSourceSignalDto
	// PositionPlanSettings are this bot's five knobs, carried in so that the round
	// can work out what it is suggesting. They are the bot's, not the rules': how
	// much money there is and how much of a move its owner can sit through are facts
	// about the machine, the same way the market it watches is.
	PositionPlanSettings PositionPlanSettingsDto
	// PositionPlan is what this round came to suggest, and HasPositionPlan whether it
	// suggests anything at all. They are filled in once, by the domain, and read by
	// both the message and the history — computing them twice would be two answers
	// that part company the moment somebody edits a setting.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
}

// StrategyBotSourceSignalDto is what one signal source said this round.
type StrategyBotSourceSignalDto struct {
	Label               string
	AggregationInterval string
	Signal              string
}

// StrategyBotRoundDecisionDto is what one round decided, before anything has been
// said or stored: the conclusion, whether it is worth a message, and whether the two
// conditions contradicted each other.
//
// Whether to send is decided here rather than left to whoever calls, because it
// depends on the last signal that reached Telegram — a fact about the bot, not about
// this round. A caller working it out for itself would need that fact handed over,
// and then two places would own the rule.
type StrategyBotRoundDecisionDto struct {
	Verdict     string
	ShouldSend  bool
	Conflicting bool
}

package dto

import "github.com/shopspring/decimal"

// PositionPlanSettingsDto is the four knobs a bot is given so that it can work out,
// every round, how big a position it is suggesting and where the exits sit.
//
// One shape serves three journeys — arriving to be saved, handed back to be read, and
// carried into a round — because all three are about the same numbers. Three shapes
// would drift, and the one that drifted would be the one a round used.
//
// **Nothing here is about borrowing.** A multiplier a caller still sends is refused
// where the bot is settled, and is not carried through this shape: this one is handed
// back out, and a figure nothing ever fills in would read as "no leverage at all" on
// every bot forever — a number on the wire that says something untrue.
//
// Every figure is an exact decimal. The two distances multiply into money, so a float
// would start drifting a price around its tenth digit — and that price is one
// somebody places an order at.
type PositionPlanSettingsDto struct {
	// Capital is the money this bot sizes against. It is the switch for the whole
	// group: without it there is nothing to stake, so the others do not apply.
	Capital decimal.Decimal
	// SizingMode is how much of that capital one opening stakes, exactly as
	// declared, and SizingValue the figure that goes with it. The spellings are the
	// replay's three and not a fourth set.
	SizingMode  string
	SizingValue decimal.Decimal
	// StopLossPercentage and TakeProfitPercentage are how far from the reference
	// price each exit sits. Either may be left out on its own.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
}

// PositionPlanDto is what one round suggests: how much to put down, and where the two
// exits sit.
//
// It carries a flag beside each optional line rather than leaving a zero to be read
// as absence. A stop-loss price of zero is a legitimate figure — a hundred percent
// away — so "no stop-loss was asked for" has to be said out loud, exactly as a round
// says whether there was a reference price to quote.
type PositionPlanDto struct {
	// Stake is what this opening puts down, and Affordable says whether the capital
	// covers it. A fixed amount larger than the capital is not a failure: the round
	// says so and carries on, the same way a replay skips one opening and continues.
	Stake      decimal.Decimal
	Affordable bool
	// StopLossPrice is where the loss is cut, and LossAtStop what reaching it costs —
	// measured against the stake, which is the whole of what this position has in the
	// market.
	StopLossPrice decimal.Decimal
	LossAtStop    decimal.Decimal
	HasStopLoss   bool
	// TakeProfitPrice and GainAtTarget are the mirror image.
	TakeProfitPrice decimal.Decimal
	GainAtTarget    decimal.Decimal
	HasTakeProfit   bool
}

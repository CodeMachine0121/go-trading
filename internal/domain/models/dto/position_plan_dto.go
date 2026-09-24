package dto

import "github.com/shopspring/decimal"

// PositionPlanSettingsDto is the knobs a bot is given so that it can work out, every
// round, how big a position it is suggesting and where the exits sit.
//
// One shape serves three journeys — arriving to be saved, handed back to be read, and
// carried into a round — because all three are about the same numbers. Three shapes
// would drift, and the one that drifted would be the one a round used.
//
// It leaves in the spelling it arrives in. Without names of its own it would leave as
// Capital and SizingMode while the request that set them said capital and sizingMode,
// and a reader matching names exactly would find none of them.
//
// Every figure is an exact decimal. The two distances multiply into money, so a float
// would start drifting a price around its tenth digit — and that price is one
// somebody places an order at.
type PositionPlanSettingsDto struct {
	// Capital is the money this bot sizes against. It is the switch for the whole
	// group: without it there is nothing to stake, so the others do not apply.
	Capital decimal.Decimal `json:"capital"`
	// SizingMode is how much of that capital one opening stakes, exactly as
	// declared, and SizingValue the figure that goes with it. The spellings are the
	// replay's three and not a fourth set.
	SizingMode  string          `json:"sizingMode"`
	SizingValue decimal.Decimal `json:"sizingValue"`
	// StopLossPercentage and TakeProfitPercentage are how far from the reference
	// price each exit sits. Either may be left out on its own.
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// Leverage is how many times its margin a contract bot's suggested position
	// carries. Only a contract bot has one; a spot bot's is zero and is left off the
	// wire altogether, because a figure nothing ever fills in would read as "no
	// leverage at all" on every spot bot forever.
	Leverage decimal.Decimal `json:"leverage,omitzero"`
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
	// StopLossPrice is where the loss is cut, and LossAtStop what reaching it costs.
	StopLossPrice decimal.Decimal
	LossAtStop    decimal.Decimal
	HasStopLoss   bool
	// TakeProfitPrice and GainAtTarget are the mirror image.
	TakeProfitPrice decimal.Decimal
	GainAtTarget    decimal.Decimal
	HasTakeProfit   bool
	// Direction is which way the suggested position faces — long or short, in the
	// position direction's own spellings. A spot suggestion only ever faces long; a
	// contract one faces whichever way its trading mode asks.
	Direction string
	// Leverage and Notional are what a contract suggestion carries: how many times its
	// margin, and how much that comes to. On a spot suggestion leverage is one and the
	// notional is the stake itself. LossAtStop and GainAtTarget are measured against
	// the notional, which is what actually moves with the price.
	Leverage decimal.Decimal
	Notional decimal.Decimal
	// LiquidatesBeforeStop warns that the stop sits beyond where the position would be
	// closed out: past the estimated liquidation price when there is one, or — when
	// there is not — a distance that times the leverage reaches a hundred percent.
	LiquidatesBeforeStop bool

	// ForContract says this suggestion is for a contract account. Everything below is
	// only ever filled in on one; a spot suggestion leaves it all at its zero value.
	ForContract bool
	// Quantity is how many units the suggestion opens, stepped down to the venue's
	// quantity step; HasQuantity is false when the venue's rules are not known yet.
	Quantity    decimal.Decimal
	HasQuantity bool
	// LiquidationPrice is where the suggested position would be closed out, estimated
	// from the reference price and on the venue's ticks. CannotBeLiquidated is a long
	// whose estimate is not above zero, and LiquidationFromSmallestTier says the
	// estimate used the specification's smallest tier for want of a full ladder.
	LiquidationPrice            decimal.Decimal
	HasLiquidationPrice         bool
	CannotBeLiquidated          bool
	LiquidationFromSmallestTier bool
	// VenueRefusal is why the venue would not take this suggestion, and when
	// HasVenueRefusal is set the suggestion carries nothing to place — no exits, no
	// quantity, no liquidation price.
	VenueRefusal    ContractOrderRefusalDto
	HasVenueRefusal bool
	// LacksTradingSpecification says the contract's specification is not recorded yet,
	// so the figures are not rounded to the venue and no liquidation price is given.
	LacksTradingSpecification bool
	// FundingRate is the rate most recently settled; FundingPayment what one settlement
	// at that rate comes to on this notional — positive is paid, negative is received.
	// FundingIntervalHours is how often it settles, zero when unknown.
	FundingRate          decimal.Decimal
	HasFundingRate       bool
	FundingPayment       decimal.Decimal
	FundingIntervalHours int
}

// ContractOrderRefusalDto is why the venue would not take a suggested order, with the
// figures that say so. Reason is one of belowMinimumQuantity, belowMinimumNotional or
// aboveTierLeverage.
type ContractOrderRefusalDto struct {
	Reason              string
	Quantity            decimal.Decimal
	MinimumQuantity     decimal.Decimal
	Notional            decimal.Decimal
	MinimumNotional     decimal.Decimal
	TierMaximumLeverage int
}

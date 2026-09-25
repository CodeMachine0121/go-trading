package dto

import "github.com/shopspring/decimal"

// PositionPlanSettingsDto is shared by save, read and round so the three cannot drift;
// figures are decimals because they multiply into order prices.
type PositionPlanSettingsDto struct {
	// Capital of zero disables the whole position plan.
	Capital decimal.Decimal `json:"capital"`
	// SizingMode uses the same spellings as the replay's position sizing modes.
	SizingMode  string          `json:"sizingMode"`
	SizingValue decimal.Decimal `json:"sizingValue"`
	// StopLossPercentage and TakeProfitPercentage are distances from the reference price and
	// each is optional.
	StopLossPercentage   decimal.Decimal `json:"stopLossPercentage"`
	TakeProfitPercentage decimal.Decimal `json:"takeProfitPercentage"`
	// Leverage applies only to contract bots and is omitted for spot bots so it does not
	// read as no leverage.
	Leverage decimal.Decimal `json:"leverage,omitzero"`
}

// PositionPlanDto uses explicit Has flags because zero is a legitimate price for an optional line.
type PositionPlanDto struct {
	// Affordable is false when the stake exceeds the capital, which is reported rather than
	// treated as a failure.
	Stake           decimal.Decimal
	Affordable      bool
	StopLossPrice   decimal.Decimal
	LossAtStop      decimal.Decimal
	HasStopLoss     bool
	TakeProfitPrice decimal.Decimal
	GainAtTarget    decimal.Decimal
	HasTakeProfit   bool
	// Direction is long or short; spot suggestions are always long.
	Direction string
	// Leverage and Notional are one and the stake on spot; LossAtStop and GainAtTarget are
	// measured against the notional.
	Leverage decimal.Decimal
	Notional decimal.Decimal
	// LiquidatesBeforeStop warns the stop lies past the estimated liquidation price, or,
	// without one, that distance × leverage reaches 100%.
	LiquidatesBeforeStop bool

	// ForContract marks a contract suggestion; the fields below are zero on spot.
	ForContract bool
	// Quantity is rounded down to the venue's quantity step; HasQuantity is false when venue
	// rules are unknown.
	Quantity    decimal.Decimal
	HasQuantity bool
	// LiquidationPrice is estimated from the reference price and rounded to the venue tick;
	// CannotBeLiquidated is a long whose estimate is not above zero.
	LiquidationPrice            decimal.Decimal
	HasLiquidationPrice         bool
	CannotBeLiquidated          bool
	LiquidationFromSmallestTier bool
	// When HasVenueRefusal is set the suggestion carries no exits, quantity or liquidation price.
	VenueRefusal    ContractOrderRefusalDto
	HasVenueRefusal bool
	// LacksTradingSpecification means figures are not rounded to the venue and no
	// liquidation price is given.
	LacksTradingSpecification bool
	// FundingRate is the latest settled rate; FundingPayment is positive when paid and
	// negative when received; FundingIntervalHours is zero when unknown.
	FundingRate          decimal.Decimal
	HasFundingRate       bool
	FundingPayment       decimal.Decimal
	FundingIntervalHours int
}

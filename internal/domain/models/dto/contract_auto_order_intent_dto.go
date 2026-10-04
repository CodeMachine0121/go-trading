package dto

import "github.com/shopspring/decimal"

// ContractAutoOrderIntentDto is what a contract round would have the bot do with real money, worked out with the round's own position plan so the order and the message agree.
type ContractAutoOrderIntentDto struct {
	// TargetPosition is long, short or flat.
	TargetPosition string
	// OpenQuantity is set only when the round has an order the venue would accept; otherwise NoOpenReason says why nothing is opened.
	OpenQuantity    decimal.Decimal
	HasOpenQuantity bool
	NoOpenReason    string
	Leverage        decimal.Decimal
	// StopLossPercentage and TakeProfitPercentage are distances from the fill price; zero places no such order.
	StopLossPercentage   decimal.Decimal
	TakeProfitPercentage decimal.Decimal
}

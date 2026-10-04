package vo

import "github.com/shopspring/decimal"

// AutoOrderPositionVo is the part of a contract position a bot opened itself and has not closed; an empty Direction is flat.
type AutoOrderPositionVo struct {
	Direction TargetPositionVo
	Quantity  decimal.Decimal
	// StopLossClientID and TakeProfitClientID name the protective orders guarding it, empty when none was placed.
	StopLossClientID   string
	TakeProfitClientID string
}

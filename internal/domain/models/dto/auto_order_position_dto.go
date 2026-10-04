package dto

import "github.com/shopspring/decimal"

// AutoOrderPositionDto is the part of a contract position the bot opened itself; Direction is empty when it holds nothing.
type AutoOrderPositionDto struct {
	Direction string          `json:"direction"`
	Quantity  decimal.Decimal `json:"quantity"`
}

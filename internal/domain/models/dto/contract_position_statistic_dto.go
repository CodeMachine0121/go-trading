package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractPositionStatisticDto is the only shape in which a position statistic leaves
// the domain. Nothing here is optional: a stored statistic is a complete one.
type ContractPositionStatisticDto struct {
	Symbol                          string          `json:"symbol"`
	StatisticTime                   time.Time       `json:"statisticTime"`
	OpenInterest                    decimal.Decimal `json:"openInterest"`
	OpenInterestValue               decimal.Decimal `json:"openInterestValue"`
	AccountLongShare                decimal.Decimal `json:"accountLongShare"`
	AccountShortShare               decimal.Decimal `json:"accountShortShare"`
	AccountLongShortRatio           decimal.Decimal `json:"accountLongShortRatio"`
	TopTraderPositionLongShare      decimal.Decimal `json:"topTraderPositionLongShare"`
	TopTraderPositionShortShare     decimal.Decimal `json:"topTraderPositionShortShare"`
	TopTraderPositionLongShortRatio decimal.Decimal `json:"topTraderPositionLongShortRatio"`
}

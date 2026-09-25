package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlementDto has a nil mark price only when the venue recorded none.
type ContractFundingRateSettlementDto struct {
	Symbol         string              `json:"symbol"`
	SettlementTime time.Time           `json:"settlementTime"`
	FundingRate    decimal.Decimal     `json:"fundingRate"`
	MarkPrice      decimal.NullDecimal `json:"markPrice"`
}

package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlementDto is the only shape in which a funding rate
// settlement leaves the domain. The mark price is null only for a settlement the
// venue recorded none for.
type ContractFundingRateSettlementDto struct {
	Symbol         string              `json:"symbol"`
	SettlementTime time.Time           `json:"settlementTime"`
	FundingRate    decimal.Decimal     `json:"fundingRate"`
	MarkPrice      decimal.NullDecimal `json:"markPrice"`
}

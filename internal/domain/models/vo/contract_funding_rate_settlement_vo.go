package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlementVo is one funding rate settlement as the venue reported
// it, already normalized. Nothing is judged here: the mark price arrives absent when
// the venue gave none, so that the domain is the one place that decides what an
// absent one means.
type ContractFundingRateSettlementVo struct {
	Symbol         string
	SettlementTime time.Time
	FundingRate    decimal.Decimal
	MarkPrice      decimal.NullDecimal
}

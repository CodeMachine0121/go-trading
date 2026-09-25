package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractFundingRateSettlementVo is one normalized funding settlement; a missing mark price stays absent so the domain decides what that means.
type ContractFundingRateSettlementVo struct {
	Symbol         string
	SettlementTime time.Time
	FundingRate    decimal.Decimal
	MarkPrice      decimal.NullDecimal
}

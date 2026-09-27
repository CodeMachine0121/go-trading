package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractTradeMarketFactsVo is what the market said while a trade was held; each part may be missing and is then reported as such, never as zero.
type ContractTradeMarketFactsVo struct {
	// FundingSettlements are the settlements inside the holding window, earliest first.
	FundingSettlements []FundingSettlementVo
	// FundingSettlementDue is true when at least one settlement time falls inside the holding window.
	FundingSettlementDue bool
	LatestPrice          decimal.Decimal
	HasLatestPrice       bool
	// ExtremesRequested is false where only a summary is wanted, so excursions are reported as not computed.
	ExtremesRequested bool
	PriceExtremes     PriceExtremesVo
}

type FundingSettlementVo struct {
	SettlementTime time.Time
	FundingRate    decimal.Decimal
	// MarkPrice is null for the venue's earliest settlements.
	MarkPrice decimal.NullDecimal
}

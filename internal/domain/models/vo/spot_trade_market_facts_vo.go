package vo

import "github.com/shopspring/decimal"

// SpotTradeMarketFactsVo is what the spot market said while a trade was held; each part may be missing and is then reported as such, never as zero.
type SpotTradeMarketFactsVo struct {
	LatestPrice    decimal.Decimal
	HasLatestPrice bool
	// ExtremesRequested is false where only a summary is wanted, so excursions are reported as not computed.
	ExtremesRequested bool
	PriceExtremes     PriceExtremesVo
}

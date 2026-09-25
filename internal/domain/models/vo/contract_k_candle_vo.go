package vo

// ContractKCandleVo is one contract bar with its funding rate and position statistics aligned; it embeds KCandleVo so spot scripts keep working, and missing figures read as zero (e.g. zero open interest likely means "not recorded").
type ContractKCandleVo struct {
	KCandleVo
	TradeCount int64
	// Mark, Index and PremiumIndex are merged across the bucket like the traded prices.
	Mark         PriceLineVo
	Index        PriceLineVo
	PremiumIndex PriceLineVo
	// FundingRate is the rate set by the most recent settlement before the close.
	FundingRate float64
	// FundingSettledInBar tells the bar that paid funding apart from bars that merely inherited the rate.
	FundingSettledInBar bool
	// The position statistic in force when this bar closed.
	OpenInterest                    float64
	OpenInterestValue               float64
	AccountLongShare                float64
	AccountShortShare               float64
	AccountLongShortRatio           float64
	TopTraderPositionLongShare      float64
	TopTraderPositionShortShare     float64
	TopTraderPositionLongShortRatio float64
}

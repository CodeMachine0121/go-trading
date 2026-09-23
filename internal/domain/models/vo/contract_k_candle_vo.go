package vo

// ContractKCandleVo is one perpetual contract bar as an indicator script sees it: the
// contract K candles of one bucket merged, with the funding rate and the position
// statistics of that stretch lined up beside them. Immutable plain data, no behavior —
// how each figure is chosen lives in ContractKCandleAlignmentDomain.
//
// It embeds KCandleVo rather than repeating its fields, so every figure a spot K
// candle has is here under the same name: a spot script moved to contract bars keeps
// every line that reads a close or a volume. The embedded candle is also reachable
// whole, as KCandleVo, for a helper written against indicator.KCandle.
//
// **A figure that is not there reads as zero**, never as a separate "is it there"
// answer: an old candle stored before the index line existed, a bar before the first
// funding settlement, a stretch with no recent position statistic. The script has to
// know that an open interest of zero most likely means "not recorded".
type ContractKCandleVo struct {
	KCandleVo
	TradeCount int64
	// Mark, Index and PremiumIndex are the three lines the venue computes beside the
	// traded prices, each merged across the bucket the same way the traded prices are.
	Mark         PriceLineVo
	Index        PriceLineVo
	PremiumIndex PriceLineVo
	// FundingRate is the rate in force when this bar closed: the one set by the most
	// recent settlement before the close.
	FundingRate float64
	// FundingSettledInBar says whether a settlement actually took place inside this
	// bar. The rate is carried on every bar, so without this a replay could not tell
	// the bar that paid from the bars that merely inherited the rate.
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

package vo

// OrderFlowPriceLevelVo is one footprint price level (named PriceLevel in a script): the lower edge of the level and what traded there.
type OrderFlowPriceLevelVo struct {
	Price float64
	// BuyVolume traded when a buyer took the offer; SellVolume when a seller hit the bid.
	BuyVolume  float64
	SellVolume float64
	TradeCount int64
}

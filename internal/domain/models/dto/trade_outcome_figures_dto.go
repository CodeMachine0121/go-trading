package dto

import "github.com/shopspring/decimal"

// TradeExcursionDto is how far the market ran against and for a trade of either journal while it was held.
type TradeExcursionDto struct {
	Available          bool            `json:"available"`
	AdversePrice       decimal.Decimal `json:"adversePrice"`
	FavorablePrice     decimal.Decimal `json:"favorablePrice"`
	AdverseProfit      decimal.Decimal `json:"adverseProfit"`
	FavorableProfit    decimal.Decimal `json:"favorableProfit"`
	AdverseRMultiple   *float64        `json:"adverseRMultiple"`
	FavorableRMultiple *float64        `json:"favorableRMultiple"`
	// UnavailableReason is noMarketData, or notComputed where only a summary was asked for.
	UnavailableReason string `json:"unavailableReason"`
}

// TradeFloatingDto is what the part still held would make at the latest price.
type TradeFloatingDto struct {
	Available bool            `json:"available"`
	Amount    decimal.Decimal `json:"amount"`
	Price     decimal.Decimal `json:"price"`
	// UnavailableReason is notOpen or noLatestPrice.
	UnavailableReason string `json:"unavailableReason"`
}

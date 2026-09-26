package dto

import "time"

// StrategyScriptDto is the owner's view and the only shape carrying the script; others get
// PublishedStrategyScriptDto.
type StrategyScriptDto struct {
	ID uint `json:"id"`
	// IsAdoptedFromMarketplace marks a copy made on adoption, whose Script is always empty.
	IsAdoptedFromMarketplace bool   `json:"isAdoptedFromMarketplace"`
	Name                     string `json:"name"`
	// Description is empty, never absent, when unset.
	Description string `json:"description"`
	Script      string `json:"script"`
	ResultType  string `json:"resultType"`
	// MarketDataKind is kCandle or contractKCandle and decides which calculation may run it.
	MarketDataKind string    `json:"marketDataKind"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	// Published is only answered to the owner.
	Published  bool                         `json:"published"`
	Parameters []StrategyScriptParameterDto `json:"parameters"`
}

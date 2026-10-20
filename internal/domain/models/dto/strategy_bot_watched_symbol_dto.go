package dto

import "github.com/shopspring/decimal"

// StrategyBotWatchedSymbolDto is one symbol a bot watches and where that symbol stands.
type StrategyBotWatchedSymbolDto struct {
	Symbol string `json:"symbol"`
	// AllocationPercentage is this symbol's share of the position plan's total capital, in percent; it may only use capital × this ÷ 100.
	AllocationPercentage decimal.Decimal `json:"allocationPercentage"`
	// LastSentSignal is empty since the last start, so the first conclusion after starting is always sent.
	LastSentSignal string `json:"lastSentSignal,omitempty"`
	// Conflicting is not a halt and clears once a round on this symbol stops conflicting.
	Conflicting bool `json:"conflicting"`
	// HaltReason is set when the system stopped watching this symbol alone; it clears on the next start.
	HaltReason string `json:"haltReason,omitempty"`
	// AutoOrderPosition is what a contract bot opened on this symbol itself and still holds; spot bots have none.
	AutoOrderPosition *AutoOrderPositionDto `json:"autoOrderPosition,omitempty"`
}

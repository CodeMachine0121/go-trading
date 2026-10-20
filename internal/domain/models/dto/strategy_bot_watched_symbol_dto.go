package dto

// StrategyBotWatchedSymbolDto is one symbol a bot watches and where that symbol stands.
type StrategyBotWatchedSymbolDto struct {
	Symbol string `json:"symbol"`
	// LastSentSignal is empty since the last start, so the first conclusion after starting is always sent.
	LastSentSignal string `json:"lastSentSignal,omitempty"`
	// Conflicting is not a halt and clears once a round on this symbol stops conflicting.
	Conflicting bool `json:"conflicting"`
	// HaltReason is set when the system stopped watching this symbol alone; it clears on the next start.
	HaltReason string `json:"haltReason,omitempty"`
	// AutoOrderPosition is what a contract bot opened on this symbol itself and still holds; spot bots have none.
	AutoOrderPosition *AutoOrderPositionDto `json:"autoOrderPosition,omitempty"`
}

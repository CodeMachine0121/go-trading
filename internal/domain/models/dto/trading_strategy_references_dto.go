package dto

// TradingStrategyReferencesDto reads running and all bot names together so rewrite and
// delete refusals see one consistent moment, and names tell the user which bot to deal with.
type TradingStrategyReferencesDto struct {
	TotalCount      int
	RunningBotNames []string
}

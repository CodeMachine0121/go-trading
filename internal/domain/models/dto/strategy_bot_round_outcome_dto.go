package dto

// StrategyBotRoundOutcomeDto is one round of a bot: what it came to on each watched symbol that took part, in the bot's order.
type StrategyBotRoundOutcomeDto struct {
	WatchedSymbolOutcomes []StrategyBotWatchedSymbolRoundOutcomeDto
}

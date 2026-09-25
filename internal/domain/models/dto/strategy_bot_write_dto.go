package dto

import "github.com/shopspring/decimal"

// StrategyBotWriteDto is unvalidated input shared by create and rewrite; ID is zero on
// create and OwnerID comes from the signed-in user.
type StrategyBotWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string
	Symbol  string
	// MarketDataKind blank means kCandle on create and unchanged on rewrite.
	MarketDataKind string
	// TradingStrategyID must name one of the caller's own trading strategies.
	TradingStrategyID      uint
	TriggerIntervalMinutes int
	// PositionPlan counts as unset when capital is empty.
	PositionPlan PositionPlanSettingsDto
	// DeclaredLeverage is validated only on input, not in the per-round plan, so bots saved
	// before the rule existed keep running; spot bots carry it only to be refused.
	DeclaredLeverage decimal.Decimal
}

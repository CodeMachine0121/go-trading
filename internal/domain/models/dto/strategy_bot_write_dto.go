package dto

import "github.com/shopspring/decimal"

// StrategyBotWriteDto is unvalidated input shared by create and rewrite; ID is zero on
// create and OwnerID comes from the signed-in user.
type StrategyBotWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string
	// Symbols is the complete list the bot watches after this write, in the owner's order.
	Symbols []string
	// MarketDataKind blank means kCandle on create and unchanged on rewrite.
	MarketDataKind string
	// TradingStrategyID must name one of the caller's own trading strategies.
	TradingStrategyID      uint
	TriggerIntervalMinutes int
	// PositionPlan counts as unset when capital is empty.
	PositionPlan PositionPlanSettingsDto
	// CapitalAllocations empty means none given: the stored shares are kept when the symbols are unchanged, otherwise the capital is split equally.
	CapitalAllocations []StrategyBotCapitalAllocationDto
	// RetainedCapitalAllocations are the stored bot's shares, filled in on rewrite.
	RetainedCapitalAllocations []StrategyBotCapitalAllocationDto
	// DeclaredLeverage is validated only on input, not in the per-round plan, so bots saved
	// before the rule existed keep running; spot bots carry it only to be refused.
	DeclaredLeverage decimal.Decimal
	// AutoOrderEntrySlippageTolerancePercentage left out means the default on create and the stored value on rewrite.
	AutoOrderEntrySlippageTolerancePercentage decimal.NullDecimal
}

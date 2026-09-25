package dto

import "time"

// StrategyBotDto includes run state so a bot list can show which bots need attention without
// extra requests.
type StrategyBotDto struct {
	ID uint `json:"id"`
	// OwnerID is never serialized; rounds use it to act as the owner since no user is signed in.
	OwnerID uint   `json:"-"`
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
	// MarketDataKind is kCandle or contractKCandle and never changes after creation.
	MarketDataKind         string `json:"marketDataKind"`
	TriggerIntervalMinutes int    `json:"triggerIntervalMinutes"`
	// PositionPlan with zero capital suggests nothing, which is how bots stored before
	// position plans read.
	PositionPlan PositionPlanSettingsDto `json:"positionPlan"`
	// NextRunAt is never serialized; a finishing round checks it so it does not overwrite a
	// restart that happened mid-round.
	NextRunAt time.Time `json:"-"`
	// TradingStrategyName is read through the association so renames are reflected.
	TradingStrategyID   uint   `json:"tradingStrategyId"`
	TradingStrategyName string `json:"tradingStrategyName"`
	RunState            string `json:"runState"`
	// LastSentSignal is empty since the last start, so the first conclusion after starting
	// is always sent.
	LastSentSignal string `json:"lastSentSignal,omitempty"`
	// HaltReason is empty when running or stopped by the owner.
	HaltReason string `json:"haltReason,omitempty"`
	// Conflicting is not a halt and clears once a round stops conflicting.
	Conflicting bool      `json:"conflicting"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

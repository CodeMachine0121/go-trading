package dto

import "time"

// StrategyBotDto is one strategy bot as it is handed back: what it is made of, and
// what it has been doing.
//
// The second half — run state, last sent signal, halt reason, conflicting — is
// carried with the first rather than fetched separately, because a list of bots is
// read to answer one question: which of these needs looking at? Answering it with a
// second round trip per bot would mean the screen either asks N more times or shows
// a list with the interesting column missing.
type StrategyBotDto struct {
	ID uint `json:"id"`
	// OwnerID is who this bot belongs to, and never leaves in a response — a
	// person reading their own bots learns nothing from being told they are theirs.
	// It is here because a round has no signed-in caller to ask: the clock started
	// it, and resolving this bot's strategies and sending its message both have to
	// be done as the person who owns it.
	OwnerID uint   `json:"-"`
	Name    string `json:"name"`
	Symbol  string `json:"symbol"`
	// TriggerIntervalMinutes is how often a running bot wakes up.
	TriggerIntervalMinutes int                          `json:"triggerIntervalMinutes"`
	SignalSources          []StrategyBotSignalSourceDto `json:"signalSources"`
	BuyCondition           StrategyBotConditionDto      `json:"buyCondition"`
	SellCondition          StrategyBotConditionDto      `json:"sellCondition"`
	RunState               string                       `json:"runState"`
	// LastSentSignal is the last signal that actually reached Telegram, and it is
	// what the next round is compared against. Empty means nothing has been sent
	// since this bot was last started, which is why the first conclusion after
	// pressing play always goes out.
	LastSentSignal string `json:"lastSentSignal,omitempty"`
	// HaltReason is why the system stopped this bot. Empty when its owner stopped
	// it, or when it is running.
	HaltReason string `json:"haltReason,omitempty"`
	// Conflicting says the last round found both conditions holding at once. It is
	// not a halt — the bot keeps running, and the mark clears itself the moment a
	// round stops conflicting — but it is the only way its owner ever finds out.
	Conflicting bool      `json:"conflicting"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

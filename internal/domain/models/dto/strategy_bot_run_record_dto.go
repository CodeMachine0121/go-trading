package dto

import "time"

// StrategyBotRunRecordDto is one of a bot's past rounds, as it is handed back.
//
// The result is one of three words and nothing else: a reader is asking whether the
// bot asked them to do something, and "it conflicted" or "the market was shut" are
// answers to a different question — one the bot's own halt reason and conflict mark
// already answer, where they can be acted on.
type StrategyBotRunRecordDto struct {
	RunNumber int       `json:"runNumber"`
	RanAt     time.Time `json:"ranAt"`
	Result    string    `json:"result"`
}

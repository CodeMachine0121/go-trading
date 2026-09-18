package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

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
	// SuggestedStake, SuggestedStopLossPrice and SuggestedTakeProfitPrice are what
	// this round suggested putting down and where it suggested getting out.
	//
	// They are pointers, and left out of the answer entirely when there was nothing
	// to suggest. That is the ordinary case — a bot with no position plan, or a round
	// that concluded nothing to open — and leaving them out means a history of such
	// rounds reads exactly as it did before suggestions existed.
	//
	// Not zero-defaulted, because a stop-loss price of zero is a legitimate figure: a
	// distance of the whole price. Absent has to be a different thing from zero.
	SuggestedStake           *decimal.Decimal `json:"suggestedStake,omitempty"`
	SuggestedStopLossPrice   *decimal.Decimal `json:"suggestedStopLossPrice,omitempty"`
	SuggestedTakeProfitPrice *decimal.Decimal `json:"suggestedTakeProfitPrice,omitempty"`
}

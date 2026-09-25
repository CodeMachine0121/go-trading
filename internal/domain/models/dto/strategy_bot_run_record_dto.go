package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// StrategyBotRunRecordDto result is limited to three words; conflicts and closed markets are
// reported by the bot's own state.
type StrategyBotRunRecordDto struct {
	RunNumber int       `json:"runNumber"`
	RanAt     time.Time `json:"ranAt"`
	Result    string    `json:"result"`
	// Suggested prices are pointers omitted when nothing was suggested, since zero is a
	// legitimate stop-loss price.
	SuggestedStake           *decimal.Decimal `json:"suggestedStake,omitempty"`
	SuggestedStopLossPrice   *decimal.Decimal `json:"suggestedStopLossPrice,omitempty"`
	SuggestedTakeProfitPrice *decimal.Decimal `json:"suggestedTakeProfitPrice,omitempty"`
	// SuggestedDirection, SuggestedLeverage and SuggestedNotional appear only on contract
	// rounds that suggested something.
	SuggestedDirection string           `json:"suggestedDirection,omitempty"`
	SuggestedLeverage  *decimal.Decimal `json:"suggestedLeverage,omitempty"`
	SuggestedNotional  *decimal.Decimal `json:"suggestedNotional,omitempty"`
}

package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// JournalLinkRoundDto is the remembered bot round a journal link names, read back only for the bot's owner.
type JournalLinkRoundDto struct {
	StrategyBotID            uint
	StrategyBotName          string
	Symbol                   string
	MarketDataKind           string
	TradingStrategyID        uint
	RunNumber                int
	RanAt                    time.Time
	Result                   string
	ReferencePrice           decimal.NullDecimal
	SuggestedStake           decimal.NullDecimal
	SuggestedQuantity        decimal.NullDecimal
	SuggestedDirection       string
	SuggestedLeverage        decimal.NullDecimal
	SuggestedStopLossPrice   decimal.NullDecimal
	SuggestedTakeProfitPrice decimal.NullDecimal
}

package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type SpotTradeRecordWriteDto struct {
	Symbol       string
	FirstBuyFill SpotTradeFillWriteDto
	Plan         SpotTradePlanWriteDto
	// Leverage and Direction are carried only so a spot trade that names either can be refused.
	Leverage          decimal.NullDecimal
	Direction         string
	TradingStrategyID *uint
	SetupTagIDs       []uint
	// JournalLinkIdentifier names the bot round the person started from, so its suggestion is copied in.
	JournalLinkIdentifier string
}

type SpotTradeFillWriteDto struct {
	Kind string
	// FilledAt nil means now, as the trading service sees it.
	FilledAt *time.Time
	Price    decimal.Decimal
	Quantity decimal.Decimal
	// Fee left null is zero; spot has no fee rates.
	Fee decimal.NullDecimal
}

type SpotTradePlanWriteDto struct {
	PlannedStopLossPrice   decimal.NullDecimal
	PlannedTakeProfitPrice decimal.NullDecimal
	EntryReason            string
	Confidence             *int
}

type SpotTradeReviewWriteDto struct {
	WentWell       string
	WentWrong      string
	NextTime       string
	ExecutionScore int
	MistakeTagIDs  []uint
}

type SpotTradeListQueryDto struct {
	Status string
	Symbol string
	Market string
	// Period narrows by first buy time; blank means no narrowing.
	Period string
	Limit  int
}

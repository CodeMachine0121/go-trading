package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ContractTradeRecordWriteDto struct {
	Symbol    string
	Direction string
	// Leverage of zero means blank, which is one.
	Leverage          decimal.Decimal
	FirstEntryFill    ContractTradeFillWriteDto
	Plan              ContractTradePlanWriteDto
	TradingStrategyID *uint
	SetupTagIDs       []uint
	// JournalLinkIdentifier names the bot round the person started from, so its suggestion is copied in.
	JournalLinkIdentifier string
}

type ContractTradeFillWriteDto struct {
	Kind string
	// FilledAt nil means now, as the trading service sees it.
	FilledAt  *time.Time
	Price     decimal.Decimal
	Quantity  decimal.Decimal
	Liquidity string
	// Fee left null is worked out from the person's fee rate.
	Fee decimal.NullDecimal
}

type ContractTradePlanWriteDto struct {
	PlannedStopLossPrice   decimal.NullDecimal
	PlannedTakeProfitPrice decimal.NullDecimal
	EntryReason            string
	Confidence             *int
}

type ContractTradeReviewWriteDto struct {
	WentWell       string
	WentWrong      string
	NextTime       string
	ExecutionScore int
	MistakeTagIDs  []uint
}

type ContractTradeListQueryDto struct {
	Status string
	Symbol string
	// Period narrows by first entry time; blank means no narrowing.
	Period string
	Limit  int
}

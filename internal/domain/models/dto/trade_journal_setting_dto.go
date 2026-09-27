package dto

import "github.com/shopspring/decimal"

// TradeJournalSettingDto leaves a rate null rather than zero when it was never set, since zero is a real rate.
type TradeJournalSettingDto struct {
	MakerFeeRate decimal.NullDecimal `json:"makerFeeRate"`
	TakerFeeRate decimal.NullDecimal `json:"takerFeeRate"`
}

// TradeJournalSettingWriteDto rates are percentages of the fill's value, e.g. 0.05 means 0.05%.
type TradeJournalSettingWriteDto struct {
	MakerFeeRate decimal.NullDecimal
	TakerFeeRate decimal.NullDecimal
}

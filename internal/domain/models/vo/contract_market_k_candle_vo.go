package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractMarketKCandleVo is one normalized, merged contract K candle; only mark/index/premium are optional because they come from separate answers, and their absence is preserved for the domain to judge.
type ContractMarketKCandleVo struct {
	Symbol              string
	OpenTime            time.Time
	Open                decimal.Decimal
	High                decimal.Decimal
	Low                 decimal.Decimal
	Close               decimal.Decimal
	Volume              decimal.Decimal
	QuoteVolume         decimal.Decimal
	TakerBuyBaseVolume  decimal.Decimal
	TakerBuyQuoteVolume decimal.Decimal
	TradeCount          int64
	MarkOpen            decimal.NullDecimal
	MarkHigh            decimal.NullDecimal
	MarkLow             decimal.NullDecimal
	MarkClose           decimal.NullDecimal
	IndexOpen           decimal.NullDecimal
	IndexHigh           decimal.NullDecimal
	IndexLow            decimal.NullDecimal
	IndexClose          decimal.NullDecimal
	PremiumIndexOpen    decimal.NullDecimal
	PremiumIndexHigh    decimal.NullDecimal
	PremiumIndexLow     decimal.NullDecimal
	PremiumIndexClose   decimal.NullDecimal
}

// ToWriteDto converts the candle for validation and storage; all rules are applied downstream.
func (contractMarketKCandleVo ContractMarketKCandleVo) ToWriteDto() dto.KCandleContractWriteDto {
	tradeCount := contractMarketKCandleVo.TradeCount

	return dto.KCandleContractWriteDto{
		Symbol:              contractMarketKCandleVo.Symbol,
		OpenTime:            contractMarketKCandleVo.OpenTime.UTC(),
		Open:                contractMarketKCandleVo.Open,
		High:                contractMarketKCandleVo.High,
		Low:                 contractMarketKCandleVo.Low,
		Close:               contractMarketKCandleVo.Close,
		Volume:              contractMarketKCandleVo.Volume,
		QuoteVolume:         decimal.NewNullDecimal(contractMarketKCandleVo.QuoteVolume),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(contractMarketKCandleVo.TakerBuyBaseVolume),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(contractMarketKCandleVo.TakerBuyQuoteVolume),
		TradeCount:          &tradeCount,
		MarkOpen:            contractMarketKCandleVo.MarkOpen,
		MarkHigh:            contractMarketKCandleVo.MarkHigh,
		MarkLow:             contractMarketKCandleVo.MarkLow,
		MarkClose:           contractMarketKCandleVo.MarkClose,
		IndexOpen:           contractMarketKCandleVo.IndexOpen,
		IndexHigh:           contractMarketKCandleVo.IndexHigh,
		IndexLow:            contractMarketKCandleVo.IndexLow,
		IndexClose:          contractMarketKCandleVo.IndexClose,
		PremiumIndexOpen:    contractMarketKCandleVo.PremiumIndexOpen,
		PremiumIndexHigh:    contractMarketKCandleVo.PremiumIndexHigh,
		PremiumIndexLow:     contractMarketKCandleVo.PremiumIndexLow,
		PremiumIndexClose:   contractMarketKCandleVo.PremiumIndexClose,
	}
}

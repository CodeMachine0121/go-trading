package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// ContractMarketKCandleVo is one contract K candle as a market source reported it,
// already normalized and already merged: the wire format and the fact that it took
// two questions to assemble both stop at the proxy and never reach the domain.
//
// The mark, index and premium index figures are the only optional ones, and they are
// optional because they are the only ones that can genuinely be missing — each comes
// from an answer of its own that may not cover the same minute. The rest arrive
// together or not at all.
//
// Nothing is judged here. "A contract K candle without a mark price is not one" is a
// rule, and rules live in the domain; this type's whole job is to make that rule
// possible to state by letting the absence survive the trip.
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

// ToWriteDto converts this reported candle into the shape the domain validates and
// stores. Nothing is judged here — every contract K candle rule is applied downstream.
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

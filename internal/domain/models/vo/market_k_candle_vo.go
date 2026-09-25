package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// MarketKCandleVo is one normalized K candle as a market source reported it.
type MarketKCandleVo struct {
	Symbol              string
	OpenTime            time.Time
	Open                decimal.Decimal
	High                decimal.Decimal
	Low                 decimal.Decimal
	Close               decimal.Decimal
	Volume              decimal.Decimal
	QuoteVolume         decimal.NullDecimal
	TakerBuyBaseVolume  decimal.NullDecimal
	TakerBuyQuoteVolume decimal.NullDecimal
}

// ToWriteDto converts the candle for validation and storage; all rules are applied downstream.
func (marketKCandleVo MarketKCandleVo) ToWriteDto() dto.KCandleWriteDto {
	return dto.KCandleWriteDto{
		Symbol:              marketKCandleVo.Symbol,
		OpenTime:            marketKCandleVo.OpenTime.UTC(),
		Open:                marketKCandleVo.Open,
		High:                marketKCandleVo.High,
		Low:                 marketKCandleVo.Low,
		Close:               marketKCandleVo.Close,
		Volume:              marketKCandleVo.Volume,
		QuoteVolume:         marketKCandleVo.QuoteVolume,
		TakerBuyBaseVolume:  marketKCandleVo.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: marketKCandleVo.TakerBuyQuoteVolume,
	}
}

package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// MarketKCandleVo is one K candle as a market source reported it, already
// normalized: the wire format stops at the proxy and never reaches the domain.
// The figures stay precise decimals because they are prices and amounts.
type MarketKCandleVo struct {
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
}

// ToWriteDto converts this reported candle into the shape the domain validates and
// stores. Nothing is judged here — every K candle rule is applied downstream.
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

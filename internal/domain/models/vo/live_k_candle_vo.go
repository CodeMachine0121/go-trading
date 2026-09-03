package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// LiveKCandleVo is one K candle as a market source is reporting it right now,
// already normalized: the wire format stops at the proxy and never reaches the
// domain. Closed says whether the interval it covers has finished, which is the
// difference between a shape that will still move and this candle's last word.
type LiveKCandleVo struct {
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
	Closed              bool
}

// ToWriteDto converts this reported candle into the shape the domain validates and
// stores. It is deliberately the same shape a fetched candle converts to, so a
// candle that closes walks into the existing rules rather than a second copy of
// them. Nothing is judged here.
func (liveKCandleVo LiveKCandleVo) ToWriteDto() dto.KCandleWriteDto {
	return dto.KCandleWriteDto{
		Symbol:              liveKCandleVo.Symbol,
		OpenTime:            liveKCandleVo.OpenTime.UTC(),
		Open:                liveKCandleVo.Open,
		High:                liveKCandleVo.High,
		Low:                 liveKCandleVo.Low,
		Close:               liveKCandleVo.Close,
		Volume:              liveKCandleVo.Volume,
		QuoteVolume:         liveKCandleVo.QuoteVolume,
		TakerBuyBaseVolume:  liveKCandleVo.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: liveKCandleVo.TakerBuyQuoteVolume,
	}
}

// ToDto hands out this candle's current shape for a viewer to look at. It is a
// separate road from ToWriteDto on purpose: what a viewer may see and what the
// system may believe are different questions, and only the second one is judged.
func (liveKCandleVo LiveKCandleVo) ToDto() dto.KCandleDto {
	return dto.KCandleDto{
		Symbol:              liveKCandleVo.Symbol,
		OpenTime:            liveKCandleVo.OpenTime.UTC(),
		Open:                liveKCandleVo.Open,
		High:                liveKCandleVo.High,
		Low:                 liveKCandleVo.Low,
		Close:               liveKCandleVo.Close,
		Volume:              liveKCandleVo.Volume,
		QuoteVolume:         liveKCandleVo.QuoteVolume,
		TakerBuyBaseVolume:  liveKCandleVo.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: liveKCandleVo.TakerBuyQuoteVolume,
	}
}

package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// LiveKCandleVo is one normalized K candle as the source reports it right now; Closed says whether its interval has finished.
type LiveKCandleVo struct {
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
	Closed              bool
}

// ToWriteDto converts to the same shape a fetched candle does, so a closing candle goes through the existing rules.
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

// ToDto is the unvalidated view for display, deliberately separate from ToWriteDto.
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

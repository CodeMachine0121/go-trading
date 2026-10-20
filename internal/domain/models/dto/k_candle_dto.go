package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type KCandleDto struct {
	Symbol              string              `json:"symbol"`
	OpenTime            time.Time           `json:"openTime"`
	Open                decimal.Decimal     `json:"open"`
	High                decimal.Decimal     `json:"high"`
	Low                 decimal.Decimal     `json:"low"`
	Close               decimal.Decimal     `json:"close"`
	Volume              decimal.Decimal     `json:"volume"`
	QuoteVolume         decimal.NullDecimal `json:"quoteVolume"`
	TakerBuyBaseVolume  decimal.NullDecimal `json:"takerBuyBaseVolume"`
	TakerBuyQuoteVolume decimal.NullDecimal `json:"takerBuyQuoteVolume"`
}

func (kCandleDto KCandleDto) ToChartDrawingAnchorCandleDto() ChartDrawingAnchorCandleDto {
	return ChartDrawingAnchorCandleDto{
		OpenTime: kCandleDto.OpenTime,
		Open:     kCandleDto.Open,
		High:     kCandleDto.High,
		Low:      kCandleDto.Low,
		Close:    kCandleDto.Close,
	}
}

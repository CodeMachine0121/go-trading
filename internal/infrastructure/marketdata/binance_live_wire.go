package marketdata

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// binanceLiveKLineMessage maps the live stream's one-letter field names.
type binanceLiveKLineMessage struct {
	KLine binanceLiveKLine `json:"k"`
}

type binanceLiveKLine struct {
	OpenTimeMilliseconds int64  `json:"t"`
	Symbol               string `json:"s"`
	Open                 string `json:"o"`
	Close                string `json:"c"`
	High                 string `json:"h"`
	Low                  string `json:"l"`
	Volume               string `json:"v"`
	QuoteVolume          string `json:"q"`
	TakerBuyBaseVolume   string `json:"V"`
	TakerBuyQuoteVolume  string `json:"Q"`
	Closed               bool   `json:"x"`

	// Declared but unread so "L" and "T" are not matched case-insensitively into "l" (low) and "t" (open time); the "T" case would silently stamp every candle one interval late.
	CloseTimeMilliseconds int64 `json:"T"`
	LastTradeNumber       int64 `json:"L"`
}

// toLiveKCandleVo converts without validation; K candle rules are applied later to closed candles only.
func (kLine binanceLiveKLine) toLiveKCandleVo() (vo.LiveKCandleVo, error) {
	figures, figureError := kLine.figures()
	if figureError != nil {
		return vo.LiveKCandleVo{}, figureError
	}

	return vo.LiveKCandleVo{
		Symbol:              kLine.Symbol,
		OpenTime:            time.UnixMilli(kLine.OpenTimeMilliseconds).UTC(),
		Open:                figures[0],
		High:                figures[1],
		Low:                 figures[2],
		Close:               figures[3],
		Volume:              figures[4],
		QuoteVolume:         decimal.NewNullDecimal(figures[5]),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(figures[6]),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(figures[7]),
		Closed:              kLine.Closed,
	}, nil
}

// figures parses every decimal in one pass so bad input fails the whole message rather than leaving a hole.
func (kLine binanceLiveKLine) figures() ([8]decimal.Decimal, error) {
	quoted := [8]string{
		kLine.Open, kLine.High, kLine.Low, kLine.Close,
		kLine.Volume, kLine.QuoteVolume, kLine.TakerBuyBaseVolume, kLine.TakerBuyQuoteVolume,
	}

	figures := [8]decimal.Decimal{}
	for index, text := range quoted {
		figure, parseError := decimal.NewFromString(text)
		if parseError != nil {
			return [8]decimal.Decimal{}, fmt.Errorf(
				"k candle figure from market source is not a number: %q", text)
		}
		figures[index] = figure
	}

	return figures, nil
}

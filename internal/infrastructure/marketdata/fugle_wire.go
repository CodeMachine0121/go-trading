package marketdata

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// fugleCandlesAnswer is the shape this source answers a candle request with. Only
// the parts the domain needs are read; whatever else it sends stops here.
type fugleCandlesAnswer struct {
	Symbol string        `json:"symbol"`
	Data   []fugleCandle `json:"data"`
}

// fugleCandle is one candle as this source states it. The figures arrive as JSON
// numbers, and they are read as text so that the exact decimal survives — a price
// that has been through a float is no longer the price that was quoted.
type fugleCandle struct {
	Date   string          `json:"date"`
	Open   decimal.Decimal `json:"open"`
	High   decimal.Decimal `json:"high"`
	Low    decimal.Decimal `json:"low"`
	Close  decimal.Decimal `json:"close"`
	Volume decimal.Decimal `json:"volume"`
}

// toMarketKCandleVo normalizes one reported candle. The three figures this venue
// does not publish on minute candles stay absent rather than becoming zeros — see
// the optional figure rules in the domain for why that difference is worth keeping.
//
// Volume is carried across exactly as reported: this venue counts in shares, and
// converting to the lots a person reads on a screen would be this system inventing a
// number nobody sent it.
func (fugleCandle fugleCandle) toMarketKCandleVo(symbol string) (vo.MarketKCandleVo, error) {
	openTime, parseError := time.Parse(time.RFC3339, fugleCandle.Date)
	if parseError != nil {
		return vo.MarketKCandleVo{}, fmt.Errorf(
			"read open time from market source for %s: %w", symbol, parseError)
	}

	return vo.MarketKCandleVo{
		Symbol:   symbol,
		OpenTime: openTime.UTC(),
		Open:     fugleCandle.Open,
		High:     fugleCandle.High,
		Low:      fugleCandle.Low,
		Close:    fugleCandle.Close,
		Volume:   fugleCandle.Volume,
	}, nil
}

// toLiveKCandleVo normalizes one pushed candle. Whether it is this candle's last
// word is not stated by the source, so it is not claimed here — deciding that is the
// live proxy's job, and it decides it by watching what arrives next.
func (fugleCandle fugleCandle) toLiveKCandleVo(symbol string) (vo.LiveKCandleVo, error) {
	openTime, parseError := time.Parse(time.RFC3339, fugleCandle.Date)
	if parseError != nil {
		return vo.LiveKCandleVo{}, fmt.Errorf(
			"read open time from market source for %s: %w", symbol, parseError)
	}

	return vo.LiveKCandleVo{
		Symbol:   symbol,
		OpenTime: openTime.UTC(),
		Open:     fugleCandle.Open,
		High:     fugleCandle.High,
		Low:      fugleCandle.Low,
		Close:    fugleCandle.Close,
		Volume:   fugleCandle.Volume,
	}, nil
}

// fugleStreamMessage is one message from the live feed. Every message names its own
// event, so one shape reads them all: the greeting, the acknowledgements, the
// heartbeats and the candles.
type fugleStreamMessage struct {
	Event   string            `json:"event"`
	Channel string            `json:"channel"`
	Data    fugleStreamCandle `json:"data"`
}

// fugleStreamCandle is a pushed candle, which unlike a fetched one names the symbol
// it belongs to — a live feed carries several markets down one connection.
type fugleStreamCandle struct {
	Symbol string `json:"symbol"`
	fugleCandle
}

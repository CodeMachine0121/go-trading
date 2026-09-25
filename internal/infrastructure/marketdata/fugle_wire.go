package marketdata

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

type fugleCandlesAnswer struct {
	Symbol string        `json:"symbol"`
	Data   []fugleCandle `json:"data"`
}

// fugleCandle reads numbers as json.Number text so exact decimals survive.
type fugleCandle struct {
	Date   string          `json:"date"`
	Open   decimal.Decimal `json:"open"`
	High   decimal.Decimal `json:"high"`
	Low    decimal.Decimal `json:"low"`
	Close  decimal.Decimal `json:"close"`
	Volume decimal.Decimal `json:"volume"`
}

// toMarketKCandleVo leaves the three figures this venue does not publish absent rather than zero, and keeps volume in shares as reported.
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

// toLiveKCandleVo does not decide finality (the live proxy does) and converts nothing, so live and stored candles match exactly.
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

// decodeFugleCandles is shared by the scheduled and live proxies so both agree on what is unreadable.
func decodeFugleCandles(body io.Reader, symbol string) (fugleCandlesAnswer, error) {
	var candlesAnswer fugleCandlesAnswer

	answer := json.NewDecoder(body)
	if decodeError := answer.Decode(&candlesAnswer); decodeError != nil {
		return fugleCandlesAnswer{}, fmt.Errorf(
			"read market source answer for %s: %w", symbol, decodeError)
	}

	// Trailing data after the value means an unreadable response, not an empty one.
	if answer.More() {
		return fugleCandlesAnswer{}, fmt.Errorf(
			"read market source answer for %s: trailing content after the answer", symbol)
	}

	return candlesAnswer, nil
}

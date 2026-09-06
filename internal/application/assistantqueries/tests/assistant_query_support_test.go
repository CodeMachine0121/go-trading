package assistantqueries_test

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// queryMaxResults is the ceiling the underlying K candle service is built with in
// these tests — large enough that it never gets in the way of what a case is
// actually checking.
const queryMaxResults = 1000

// at is a moment on 2026-08-29, given only as an hour and a minute because that is
// all any case here cares about.
func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

// kCandleAt is one finished candle at a moment, with only its close price varying —
// every other field is filled with a plausible fixed value.
func kCandleAt(openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.RequireFromString("1200"),
		TakerBuyBaseVolume:  decimal.RequireFromString("5"),
		TakerBuyQuoteVolume: decimal.RequireFromString("600"),
	}
}

// indicatorNow is the moment every calculation below is asked at. It sits on a
// five-minute edge, so the candle at 09:10 belongs to a bucket that has finished.
var indicatorNow = at(9, 15)

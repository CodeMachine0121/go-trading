package assistantqueries_test

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// queryMaxResults is large enough never to interfere with what a case checks.
const queryMaxResults = 1000

// at is a moment on 2026-08-29.
func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

func kCandleAt(openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
	}
}

// indicatorNow sits on a five-minute edge, so the 09:10 candle's bucket has finished.
var indicatorNow = at(9, 15)

// assistantViewerID owns every strategy script these tests save or read, so they test capabilities rather than visibility.
const assistantViewerID = uint(1)

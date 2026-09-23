package marketdata

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// twseRealtimeAnswer is the shape this source answers a quote request with. It
// answers about every symbol asked for in one body, so the entries are read by the
// code each carries rather than by their position.
type twseRealtimeAnswer struct {
	Quotes []twseRealtimeQuote `json:"msgArray"`
	// ReturnCode is this source's own verdict on the request. A body can arrive
	// perfectly well formed and still be a refusal, which is not something an HTTP
	// status alone would have said.
	ReturnCode    string `json:"rtcode"`
	ReturnMessage string `json:"rtmessage"`
}

// twseRealtimeOkReturnCode is what this source says when it answered the question.
const twseRealtimeOkReturnCode = "0000"

// twseRealtimeQuote is one symbol as this source states it right now.
//
// Every figure arrives as text, which is kept rather than fought: a price that has
// been through a float is no longer the price that was quoted, and this source sends
// a bare dash for figures it has nothing to say about — something no numeric type
// can hold.
//
// Asking for a symbol under the wrong board answers with an entry carrying nothing at
// all, so an entry with no code is how "that is not where this one is listed" arrives.
// It is an answer, not a failure.
type twseRealtimeQuote struct {
	// Symbol is the code this entry is about; empty means the entry is a placeholder
	// for a symbol this board does not list.
	Symbol string `json:"c"`
	// LatestPrice is the most recent trade, or a dash before the first one of the day.
	LatestPrice string `json:"z"`
	// CumulativeVolume is the day's running total, not this minute's — the one
	// difference from the history source, and the one toLiveKCandleVo cannot correct
	// on its own. The **unit needs no correcting**: this venue and the history source
	// both count in lots. Measured on 2026-09-22, this field and the history source's
	// minute volumes summed to the same 18876 for 2330, to the unit.
	CumulativeVolume string `json:"v"`
	// LatestTradeTime is when that trade happened, in this market's own zone. It is
	// not the moment the source published the answer — a quote republished unchanged
	// carries the old trade's time, which is exactly what says no trade has happened
	// since.
	LatestTradeTime string `json:"t"`
	// TradingDate is the day LatestTradeTime falls on, as this source spells it.
	TradingDate string `json:"d"`
}

// toLiveKCandleVo normalizes one quote into the shape the rest of the system works
// in, correcting the two things this source says differently from the one the
// history comes from.
//
// **A quote with nothing to say comes back as no candle rather than as an error.**
// Two of those arrive constantly and neither is a fault: an entry for the board a
// code is not listed on carries nothing at all, and a stock that has not traded today
// carries a dash where its price would be. Calling either a failure to read would
// bury the genuine unreadable answer under a log line every few seconds, per symbol,
// all session — and reading the dash as a number would put a candle at zero on
// somebody's chart.
//
// **Volume is carried across exactly as reported.** Both Taiwan sources count in the
// same unit, so converting would be this system inventing a number nobody sent it —
// and it would produce the very thousandfold split between a stock's history and its
// live updates that a conversion here was once believed to prevent. Nothing would
// raise an error either way, because both numbers are perfectly valid volumes; only
// the judgments that read volume would quietly stop meaning anything.
//
// **The volume carried out is still the day's running total.** Turning that into one
// minute's worth needs to remember where the minute started, which is a thing a
// conversion cannot know and therefore does not pretend to; twseFormingKCandle owns
// it.
//
// The four prices are all the latest trade. One quote is a point, not a bar — the
// highs and lows of a minute emerge from the points that fall inside it, and taking
// this source's own high and low would take the whole day's.
func (twseRealtimeQuote twseRealtimeQuote) toLiveKCandleVo(
	marketZone *time.Location,
) (vo.LiveKCandleVo, bool, error) {
	latestPrice, priceError := decimal.NewFromString(
		strings.TrimSpace(twseRealtimeQuote.LatestPrice))
	if strings.TrimSpace(twseRealtimeQuote.Symbol) == "" || priceError != nil {
		return vo.LiveKCandleVo{}, false, nil
	}

	cumulativeVolume, volumeError := decimal.NewFromString(
		strings.TrimSpace(twseRealtimeQuote.CumulativeVolume))
	if volumeError != nil {
		return vo.LiveKCandleVo{}, false, fmt.Errorf(
			"read cumulative volume from market source for %s: %w",
			twseRealtimeQuote.Symbol, volumeError)
	}

	tradeTime, timeError := twseRealtimeQuote.tradeTimeIn(marketZone)
	if timeError != nil {
		return vo.LiveKCandleVo{}, false, timeError
	}

	return vo.LiveKCandleVo{
		Symbol:   twseRealtimeQuote.Symbol,
		OpenTime: tradeTime.UTC(),
		Open:     latestPrice,
		High:     latestPrice,
		Low:      latestPrice,
		Close:    latestPrice,
		Volume:   cumulativeVolume,
	}, true, nil
}

// tradeTimeIn is when the trade this quote reports happened, read in the market's own
// zone.
//
// The date and the time of day arrive separately because that is how this source
// sends them, and they are put together here rather than anywhere further in: a time
// of day with no date is a thing that cannot be compared to anything, and passing one
// on would make every later reader guess at the day.
func (twseRealtimeQuote twseRealtimeQuote) tradeTimeIn(
	marketZone *time.Location,
) (time.Time, error) {
	tradeTime, parseError := time.ParseInLocation(
		"20060102 15:04:05",
		strings.TrimSpace(twseRealtimeQuote.TradingDate)+" "+
			strings.TrimSpace(twseRealtimeQuote.LatestTradeTime),
		marketZone)
	if parseError != nil {
		return time.Time{}, fmt.Errorf(
			"read trade time from market source for %s: %w",
			twseRealtimeQuote.Symbol, parseError)
	}

	return tradeTime, nil
}

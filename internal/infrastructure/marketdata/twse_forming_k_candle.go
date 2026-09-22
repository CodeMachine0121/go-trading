package marketdata

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// twseCandleInterval is the length one K candle covers. This source is asked for a
// price, not for a bar, so the length is entirely ours to impose — which is why it is
// read from the domain rather than written down here.
const twseCandleInterval = domains.KCandleInterval

// twseFormingKCandle turns a stream of quotes into candles of the one length the
// system works in, for a single symbol.
//
// It exists because this source answers "what is it now", never "what happened in
// the last minute". Two things therefore have to be worked out from the quotes
// themselves, and both need memory of what came before — which is why they live here
// and not in the conversion, where a single quote is all there is to look at.
//
// **Which minute a quote belongs to** is decided by the time of the trade it reports,
// never by when the answer arrived. A source that is a few seconds behind would
// otherwise push a trade into the minute after the one it happened in, and the stored
// candle would be wrong rather than merely late.
//
// **How much traded in that minute** is the day's running total now, less what it
// stood at when the minute opened. The running total is all this source publishes, so
// subtraction is the only way to a minute's worth — and getting it wrong is silent:
// a minute carrying the whole day's volume is still a perfectly plausible number.
//
// A quote landing in a later minute is what proves the earlier one finished. The last
// minute of a session is therefore never reported finished here, which costs nothing:
// the scheduled round collects it, and that is what the round is for.
type twseFormingKCandle struct {
	symbol string
	// slotOpenTime is the minute currently being built. Zero means none yet.
	slotOpenTime time.Time
	// openingCumulativeVolume is the day's running total as it stood when this minute
	// opened, which is what this minute's volume is measured from.
	openingCumulativeVolume decimal.Decimal
	// latestCumulativeVolume is the running total as last reported, which becomes the
	// next minute's opening figure the moment this one finishes.
	latestCumulativeVolume decimal.Decimal
	openPrice              decimal.Decimal
	highPrice              decimal.Decimal
	lowPrice               decimal.Decimal
	closePrice             decimal.Decimal
	// latestTradeTime is the trade this slot last heard about, so that a quote
	// republished unchanged is recognised as saying nothing new.
	latestTradeTime time.Time
}

func newTwseFormingKCandle(symbol string) *twseFormingKCandle {
	return &twseFormingKCandle{symbol: symbol}
}

// absorb takes one quote and reports what should be passed on: the minute it belongs
// to as it now stands, and — when the quote proves an earlier minute finished — that
// earlier minute's last word first.
//
// The finished one is reported first on purpose. It is the one that gets stored, and
// a viewer who saw the new minute before the old one's closing figures would watch
// the chart go backwards.
//
// A minute nothing traded in is not reported at all, in progress or finished. Zero is
// a real volume and this source has no way to say "nothing happened" other than by
// repeating itself, so a minute whose running total never moved is a minute with no
// candle — which is the same thing the history source says about it by leaving it out.
func (twseFormingKCandle *twseFormingKCandle) absorb(
	quotedKCandle vo.LiveKCandleVo,
) []vo.LiveKCandleVo {
	tradeTime := quotedKCandle.OpenTime
	slotOpenTime := tradeTime.Truncate(twseCandleInterval)

	// A quote older than the minute being built says nothing new — the source
	// restating something already superseded, or answers arriving out of order.
	if !twseFormingKCandle.slotOpenTime.IsZero() &&
		slotOpenTime.Before(twseFormingKCandle.slotOpenTime) {
		return nil
	}

	// The same trade, restated. This source republishes its answer every few seconds
	// whether or not anything happened, so recognising that costs one comparison and
	// saves a candle being re-announced several times a minute.
	if tradeTime.Equal(twseFormingKCandle.latestTradeTime) &&
		quotedKCandle.Volume.Equal(twseFormingKCandle.latestCumulativeVolume) {
		return nil
	}

	reportedKCandles := make([]vo.LiveKCandleVo, 0, 2)

	if twseFormingKCandle.slotOpenTime.IsZero() {
		twseFormingKCandle.startSlot(slotOpenTime, quotedKCandle)
	} else if slotOpenTime.After(twseFormingKCandle.slotOpenTime) {
		if finishedKCandle, hasFinished := twseFormingKCandle.finishedSlot(); hasFinished {
			reportedKCandles = append(reportedKCandles, finishedKCandle)
		}

		twseFormingKCandle.startSlot(slotOpenTime, quotedKCandle)
	} else {
		twseFormingKCandle.foldIntoSlot(quotedKCandle)
	}

	twseFormingKCandle.latestTradeTime = tradeTime

	if formingKCandle, isForming := twseFormingKCandle.formingSlot(); isForming {
		reportedKCandles = append(reportedKCandles, formingKCandle)
	}

	return reportedKCandles
}

// startSlot begins a new minute, measuring its volume from wherever the running total
// stood at the end of the last one.
func (twseFormingKCandle *twseFormingKCandle) startSlot(
	slotOpenTime time.Time, quotedKCandle vo.LiveKCandleVo,
) {
	twseFormingKCandle.openingCumulativeVolume = twseFormingKCandle.latestCumulativeVolume
	if twseFormingKCandle.slotOpenTime.IsZero() {
		// Nothing to measure from: this is the first quote ever seen for this symbol.
		// Measuring from itself means the first minute counts only what traded after
		// we started listening, which is the honest answer to a question we arrived
		// too late to answer fully.
		twseFormingKCandle.openingCumulativeVolume = quotedKCandle.Volume
	}

	twseFormingKCandle.slotOpenTime = slotOpenTime
	twseFormingKCandle.recordCumulativeVolume(quotedKCandle.Volume)
	twseFormingKCandle.openPrice = quotedKCandle.Close
	twseFormingKCandle.highPrice = quotedKCandle.Close
	twseFormingKCandle.lowPrice = quotedKCandle.Close
	twseFormingKCandle.closePrice = quotedKCandle.Close
}

// foldIntoSlot adds one more quote to the minute being built. The prices of a minute
// are the shape of the points inside it: where it started, how far it reached either
// way, and where it stands now.
func (twseFormingKCandle *twseFormingKCandle) foldIntoSlot(quotedKCandle vo.LiveKCandleVo) {
	if quotedKCandle.Close.GreaterThan(twseFormingKCandle.highPrice) {
		twseFormingKCandle.highPrice = quotedKCandle.Close
	}
	if quotedKCandle.Close.LessThan(twseFormingKCandle.lowPrice) {
		twseFormingKCandle.lowPrice = quotedKCandle.Close
	}

	twseFormingKCandle.closePrice = quotedKCandle.Close
	twseFormingKCandle.recordCumulativeVolume(quotedKCandle.Volume)
}

// recordCumulativeVolume takes in the day's running total as this source last stated
// it, and is the only place that total is ever written.
//
// It exists because the rule guarding it has to hold on both ways in — opening a
// minute and adding to one — and written at each of them it would be two rules that
// only look like one. **A running total below what this minute is measured from means
// the count restarted**: a new day, or the source starting over. The figure in hand is
// the truth from there, and carrying the old opening figure forward would make the
// minute's volume negative, which is a number nothing downstream is built to
// disbelieve.
func (twseFormingKCandle *twseFormingKCandle) recordCumulativeVolume(
	cumulativeVolume decimal.Decimal,
) {
	if cumulativeVolume.LessThan(twseFormingKCandle.openingCumulativeVolume) {
		twseFormingKCandle.openingCumulativeVolume = cumulativeVolume
	}

	twseFormingKCandle.latestCumulativeVolume = cumulativeVolume
}

// formingSlot is the minute as it currently stands, if anything traded in it.
func (twseFormingKCandle *twseFormingKCandle) formingSlot() (vo.LiveKCandleVo, bool) {
	return twseFormingKCandle.slotKCandle(false)
}

// finishedSlot is the minute's last word, if anything traded in it.
func (twseFormingKCandle *twseFormingKCandle) finishedSlot() (vo.LiveKCandleVo, bool) {
	return twseFormingKCandle.slotKCandle(true)
}

// slotKCandle builds the minute being held, and says whether there is one to build.
//
// It is one method rather than two because the only difference between a minute's
// current shape and its last word is which of them it is said to be — and written
// twice, the volume subtraction would be in two places, which is one more than a rule
// this quiet should ever be in.
func (twseFormingKCandle *twseFormingKCandle) slotKCandle(
	isClosed bool,
) (vo.LiveKCandleVo, bool) {
	slotVolume := twseFormingKCandle.latestCumulativeVolume.Sub(
		twseFormingKCandle.openingCumulativeVolume)
	if !slotVolume.IsPositive() {
		return vo.LiveKCandleVo{}, false
	}

	return vo.LiveKCandleVo{
		Symbol:   twseFormingKCandle.symbol,
		OpenTime: twseFormingKCandle.slotOpenTime,
		Open:     twseFormingKCandle.openPrice,
		High:     twseFormingKCandle.highPrice,
		Low:      twseFormingKCandle.lowPrice,
		Close:    twseFormingKCandle.closePrice,
		Volume:   slotVolume,
		Closed:   isClosed,
	}, true
}

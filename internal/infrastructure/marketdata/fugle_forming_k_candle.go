package marketdata

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// fugleFormingKCandle folds what this source pushes into candles of the length the
// system works in, and decides which of them is a candle's last word.
//
// It exists because this source's live feed says neither of those things. It does not
// say how long a pushed candle covers — its subscription takes no interval — and it
// does not say when one is final. Believing a push as it stands would store bars of
// an unknown length and never mark any of them closed.
//
// Both are worked out from the pushes themselves. A push is placed in the
// five-minute slot its own time falls in, replacing whatever that slot last held from
// the same source time; and a push landing in a later slot is what proves the earlier
// slot finished. That reading is right whether the source pushes one bar a minute or
// one every five: at five minutes each slot has a single contributor and folding is
// the identity.
//
// The last candle of a session is therefore never reported closed here — nothing
// arrives after it to prove it finished. That is deliberate and costs nothing: the
// five-minute round collects it, which is the very thing the round is for.
type fugleFormingKCandle struct {
	symbol       string
	slotOpenTime time.Time
	// contributions is what each distinct source time last reported, so that a repeat
	// of the same bar replaces it rather than being counted twice.
	contributions map[time.Time]vo.LiveKCandleVo
	// order is the source times in the order they first arrived, which is what decides
	// the slot's opening and closing prices.
	order []time.Time
}

func newFugleFormingKCandle(symbol string) *fugleFormingKCandle {
	return &fugleFormingKCandle{symbol: symbol, contributions: make(map[time.Time]vo.LiveKCandleVo)}
}

// absorb takes one pushed candle and reports what should be passed on: the slot it
// belongs to as it now stands, and — when the push proves an earlier slot finished —
// that earlier slot's last word first.
//
// The finished one is reported first on purpose. It is the one that gets stored, and
// a viewer who saw the new slot before the old one's closing figures would watch the
// chart go backwards.
func (fugleFormingKCandle *fugleFormingKCandle) absorb(
	reportedKCandle vo.LiveKCandleVo,
) []vo.LiveKCandleVo {
	slotOpenTime := reportedKCandle.OpenTime.Truncate(fugleCandleInterval)

	// A push older than the slot being built says nothing new — the source restating
	// something already superseded, or messages arriving out of order.
	if !fugleFormingKCandle.slotOpenTime.IsZero() && slotOpenTime.Before(fugleFormingKCandle.slotOpenTime) {
		return nil
	}

	liveKCandles := make([]vo.LiveKCandleVo, 0, 2)

	if !fugleFormingKCandle.slotOpenTime.IsZero() && slotOpenTime.After(fugleFormingKCandle.slotOpenTime) {
		closedKCandle := fugleFormingKCandle.folded()
		closedKCandle.Closed = true
		liveKCandles = append(liveKCandles, closedKCandle)
		fugleFormingKCandle.reset()
	}

	fugleFormingKCandle.slotOpenTime = slotOpenTime
	if _, hasContributed := fugleFormingKCandle.contributions[reportedKCandle.OpenTime]; !hasContributed {
		fugleFormingKCandle.order = append(fugleFormingKCandle.order, reportedKCandle.OpenTime)
	}
	fugleFormingKCandle.contributions[reportedKCandle.OpenTime] = reportedKCandle

	return append(liveKCandles, fugleFormingKCandle.folded())
}

// folded is the slot as it now stands: it opened where its earliest contribution
// opened, closed where its latest one closed, reached as high and as low as any of
// them, and traded everything they all traded.
func (fugleFormingKCandle *fugleFormingKCandle) folded() vo.LiveKCandleVo {
	foldedKCandle := vo.LiveKCandleVo{
		Symbol:   fugleFormingKCandle.symbol,
		OpenTime: fugleFormingKCandle.slotOpenTime,
	}

	for index, sourceTime := range fugleFormingKCandle.order {
		contribution := fugleFormingKCandle.contributions[sourceTime]

		if index == 0 {
			foldedKCandle.Open = contribution.Open
			foldedKCandle.High = contribution.High
			foldedKCandle.Low = contribution.Low
		}
		if contribution.High.GreaterThan(foldedKCandle.High) {
			foldedKCandle.High = contribution.High
		}
		if contribution.Low.LessThan(foldedKCandle.Low) {
			foldedKCandle.Low = contribution.Low
		}

		foldedKCandle.Close = contribution.Close
		foldedKCandle.Volume = foldedKCandle.Volume.Add(contribution.Volume)
	}

	return foldedKCandle
}

func (fugleFormingKCandle *fugleFormingKCandle) reset() {
	fugleFormingKCandle.contributions = make(map[time.Time]vo.LiveKCandleVo)
	fugleFormingKCandle.order = nil
}

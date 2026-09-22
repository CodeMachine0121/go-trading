package marketdata

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// twseQuoteFollow is one follow of this venue while it is running: the symbols it
// covers, a folder holding each one's minute in progress, and the line the candles
// leave by.
//
// It exists because those three travel together through every step of a follow and
// mean nothing apart — a folder with no line to report to, or a line with no folders
// behind it, is not a thing the system ever has. Passed separately they were three
// arguments threaded through every method, which is the shape a missing object takes.
//
// It holds a running job rather than a domain concept, so it lives beside the proxy
// that runs it and carries no model suffix.
type twseQuoteFollow struct {
	// channel is what this follow covers, and what it is called in a log line.
	channel vo.LiveFollowChannelVo
	// formingBySymbol is one folder per symbol. They cannot share one: folding is
	// about which minute a quote falls in and what the running total stood at when
	// that minute opened, and two symbols answering alternately would each keep
	// resetting the other's.
	formingBySymbol map[string]*twseFormingKCandle
	// marketZone is the zone this source states its trade times in. A time of day
	// with no zone names a different moment in every reader.
	marketZone   *time.Location
	liveKCandles chan<- vo.LiveKCandleVo
}

func newTwseQuoteFollow(
	channel vo.LiveFollowChannelVo,
	marketZone *time.Location,
	liveKCandles chan<- vo.LiveKCandleVo,
) *twseQuoteFollow {
	formingBySymbol := make(map[string]*twseFormingKCandle, len(channel.Symbols))
	for _, symbol := range channel.Symbols {
		formingBySymbol[symbol] = newTwseFormingKCandle(symbol)
	}

	return &twseQuoteFollow{
		channel:         channel,
		formingBySymbol: formingBySymbol,
		marketZone:      marketZone,
		liveKCandles:    liveKCandles,
	}
}

// end closes the line the candles leave by, which is the one way a caller ever learns
// that this follow is over.
//
// It belongs to the follow rather than to whoever is running it because the follow is
// what owns the line. Closed from outside, every way a follow can end would have to
// remember to do it, and the one that forgot would leave a reader waiting for ever.
func (twseQuoteFollow *twseQuoteFollow) end() {
	close(twseQuoteFollow.liveKCandles)
}

// publish folds one answer into the candles it changes and sends them on, reporting
// whether the follow should carry on.
//
// A quote about a symbol this follow never asked for belongs to nobody here. Whether
// a quote says anything at all is the quote's own to answer — see toLiveKCandleVo for
// the two everyday ways it says nothing.
func (twseQuoteFollow *twseQuoteFollow) publish(
	executionContext context.Context, quotes []twseRealtimeQuote,
) bool {
	for _, quote := range quotes {
		forming, isFollowed := twseQuoteFollow.formingBySymbol[quote.Symbol]
		if !isFollowed {
			continue
		}

		quotedKCandle, hasQuote, convertError := quote.toLiveKCandleVo(
			twseQuoteFollow.marketZone)
		if convertError != nil {
			log.Printf("live market data: unreadable quote: %v", convertError)

			continue
		}
		if !hasQuote {
			continue
		}

		for _, liveKCandle := range forming.absorb(quotedKCandle) {
			select {
			case twseQuoteFollow.liveKCandles <- liveKCandle:
			case <-executionContext.Done():
				return false
			}
		}
	}

	return true
}

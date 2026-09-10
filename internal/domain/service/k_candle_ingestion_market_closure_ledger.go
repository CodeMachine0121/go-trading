package service

import (
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleIngestionMarketClosureLedger remembers which markets have already been
// decided shut, and for which of their own days.
//
// It is the one place that memory lives, and it holds nothing else. Whether a market
// looks shut is read from what a round saw; when the decision expires is the
// market's own calendar; this only remembers the conclusion in between. Keeping
// those three apart is what leaves the ingestion service with no shared state of its
// own to lock.
//
// The day is carried rather than the date it was written on, because "still today"
// is a question about the market's calendar and not about how long ago it was: a
// round at eleven at night in Taipei is the same trading day as one at ten in the
// morning, and midnight somewhere else has nothing to do with it.
type kCandleIngestionMarketClosureLedger struct {
	mutex        sync.Mutex
	closedOnDate map[vo.MarketVo]time.Time
}

func newKCandleIngestionMarketClosureLedger() *kCandleIngestionMarketClosureLedger {
	return &kCandleIngestionMarketClosureLedger{
		closedOnDate: make(map[vo.MarketVo]time.Time),
	}
}

// presumeClosed records that this market is shut for the trading day given.
func (marketClosureLedger *kCandleIngestionMarketClosureLedger) presumeClosed(
	market vo.MarketVo, tradingDate time.Time,
) {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	marketClosureLedger.closedOnDate[market] = tradingDate
}

// reconsider forgets whatever was decided about this market, so the next thing that
// asks about it asks the source instead of the memory.
//
// It exists because the decision is an inference, and an inference can be wrong: a
// source that publishes late empties every symbol at once, which is the same shape as
// a holiday. Somebody asking for a symbol by hand is somebody saying they want it
// asked — so their request clears the decision rather than being turned away by it,
// and a market that really is shut simply gets decided shut again.
func (marketClosureLedger *kCandleIngestionMarketClosureLedger) reconsider(
	market vo.MarketVo,
) {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	delete(marketClosureLedger.closedOnDate, market)
}

// isPresumedClosed reports a market already decided shut for the trading day given.
// Any other day is a fresh judgement — a holiday is one day off, not a verdict.
func (marketClosureLedger *kCandleIngestionMarketClosureLedger) isPresumedClosed(
	market vo.MarketVo, tradingDate time.Time,
) bool {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	closedOnDate, wasPresumedClosed := marketClosureLedger.closedOnDate[market]

	return wasPresumedClosed && closedOnDate.Equal(tradingDate)
}

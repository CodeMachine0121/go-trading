package service

import (
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// marketClosureLedger remembers which markets have already been decided shut, and
// for which stretch of their trading.
//
// It is the one place that memory lives, and it holds nothing else. Whether a market
// looks shut is read from what a round saw; when the decision expires is which stretch
// of trading the next round is working on; this only remembers the conclusion in
// between. Keeping those three apart is what leaves the ingestion service with no
// shared state of its own to lock.
//
// **The stretch is remembered rather than the day.** A venue may trade twice a day,
// and the two are separately capable of being shut: an exchange can cancel a day board
// and still hold its evening board that night. Remembering a day would let the day
// board's silence speak for the evening board, and the system would give up on trading
// it never asked about. A stretch's silence is only that stretch's.
//
// The stretch is carried as the moment it began, which names it exactly once: no two
// stretches of one market ever start together, and "still the same stretch" is then a
// question nobody has to ask a calendar.
type marketClosureLedger struct {
	mutex                 sync.Mutex
	closedOccurrenceStart map[vo.MarketVo]time.Time
}

func newMarketClosureLedger() *marketClosureLedger {
	return &marketClosureLedger{closedOccurrenceStart: make(map[vo.MarketVo]time.Time)}
}

// presumeClosed records that this market is shut for the stretch of trading given.
func (marketClosureLedger *marketClosureLedger) presumeClosed(
	market vo.MarketVo, sessionOccurrence vo.TradingSessionOccurrenceVo,
) {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	marketClosureLedger.closedOccurrenceStart[market] = sessionOccurrence.StartTime
}

// reconsider forgets whatever was decided about this market, so the next thing that
// asks about it asks the source instead of the memory.
//
// It exists because the decision is an inference, and an inference can be wrong: a
// source that publishes late empties every symbol at once, which is the same shape as
// a holiday. Somebody asking for a symbol by hand is somebody saying they want it
// asked — so their request clears the decision rather than being turned away by it,
// and a market that really is shut simply gets decided shut again.
func (marketClosureLedger *marketClosureLedger) reconsider(market vo.MarketVo) {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	delete(marketClosureLedger.closedOccurrenceStart, market)
}

// isPresumedClosed reports a market already decided shut for the stretch of trading
// given. Any other stretch is a fresh judgement — a holiday is one board off, not a
// verdict.
func (marketClosureLedger *marketClosureLedger) isPresumedClosed(
	market vo.MarketVo, sessionOccurrence vo.TradingSessionOccurrenceVo,
) bool {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	closedOccurrenceStart, wasPresumedClosed := marketClosureLedger.closedOccurrenceStart[market]

	return wasPresumedClosed && closedOccurrenceStart.Equal(sessionOccurrence.StartTime)
}

package service

import (
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// kCandleIngestionMarketClosureLedger remembers which markets were judged shut for which of their own trading days (not wall-clock dates), and holds nothing else.
type kCandleIngestionMarketClosureLedger struct {
	mutex        sync.Mutex
	closedOnDate map[vo.MarketVo]time.Time
}

func newKCandleIngestionMarketClosureLedger() *kCandleIngestionMarketClosureLedger {
	return &kCandleIngestionMarketClosureLedger{
		closedOnDate: make(map[vo.MarketVo]time.Time),
	}
}

func (marketClosureLedger *kCandleIngestionMarketClosureLedger) presumeClosed(
	market vo.MarketVo, tradingDate time.Time,
) {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	marketClosureLedger.closedOnDate[market] = tradingDate
}

// reconsider forgets the closure decision, since it is only an inference (a late source looks like a holiday) and a manual request should re-ask the source.
func (marketClosureLedger *kCandleIngestionMarketClosureLedger) reconsider(
	market vo.MarketVo,
) {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	delete(marketClosureLedger.closedOnDate, market)
}

// isPresumedClosed reports whether the market was judged shut for this trading day; any other day is judged afresh.
func (marketClosureLedger *kCandleIngestionMarketClosureLedger) isPresumedClosed(
	market vo.MarketVo, tradingDate time.Time,
) bool {
	marketClosureLedger.mutex.Lock()
	defer marketClosureLedger.mutex.Unlock()

	closedOnDate, wasPresumedClosed := marketClosureLedger.closedOnDate[market]

	return wasPresumedClosed && closedOnDate.Equal(tradingDate)
}

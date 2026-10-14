package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// OrderFlowMinuteLedgerDomain tallies trades, arriving oldest first, into minute × price level; it only grows while one fetch streams through it, then hands its minutes to a ContractOrderFlowDayDomain.
type OrderFlowMinuteLedgerDomain struct {
	priceStep OrderFlowPriceStepDomain
	// tallies is minute open (Unix seconds) → level → tally.
	tallies         map[int64]map[int64]*orderFlowTally
	latestTradeTime time.Time
}

// orderFlowTally is what traded at one level in one minute.
type orderFlowTally struct {
	buyVolume  float64
	sellVolume float64
	tradeCount int64
}

func NewOrderFlowMinuteLedgerDomain(priceStep OrderFlowPriceStepDomain) *OrderFlowMinuteLedgerDomain {
	return &OrderFlowMinuteLedgerDomain{
		priceStep: priceStep,
		tallies:   map[int64]map[int64]*orderFlowTally{},
	}
}

// Record files a trade under the minute it happened in (a trade at 09:01:00.000 opens 09:01) and the level its price rounds down to.
func (ledgerDomain *OrderFlowMinuteLedgerDomain) Record(trade vo.ContractAggregatedTradeVo) {
	tradeTime := trade.TradeTime.UTC()
	minuteOpen := tradeTime.Truncate(time.Minute).Unix()
	levels, hasMinute := ledgerDomain.tallies[minuteOpen]
	if !hasMinute {
		levels = map[int64]*orderFlowTally{}
		ledgerDomain.tallies[minuteOpen] = levels
	}

	level := ledgerDomain.priceStep.LevelOf(trade.Price)
	tally, hasLevel := levels[level]
	if !hasLevel {
		tally = &orderFlowTally{}
		levels[level] = tally
	}

	if trade.BuyerIsTaker {
		tally.buyVolume += trade.Quantity
	} else {
		tally.sellVolume += trade.Quantity
	}
	tally.tradeCount += max(trade.TradeCount, 1)

	if tradeTime.After(ledgerDomain.latestTradeTime) {
		ledgerDomain.latestTradeTime = tradeTime
	}
}

// CompleteUntil is how far an interrupted fetch can be trusted: trades arrive oldest first, so every minute before the latest trade's minute is whole, and that minute may not be. Without trades it is the given start.
func (ledgerDomain *OrderFlowMinuteLedgerDomain) CompleteUntil(start time.Time) time.Time {
	latestMinute := ledgerDomain.latestTradeTime.Truncate(time.Minute)
	if latestMinute.After(start) {
		return latestMinute
	}

	return start
}

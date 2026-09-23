package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// TradeOutcomeVo is one finished round trip as the trade statistics read it — the
// one shape a spot trade and a contract trade both turn into, so the statistics are
// worked out once for both. Immutable, no behavior.
type TradeOutcomeVo struct {
	// NetProfit is what the round trip made after everything it paid.
	NetProfit decimal.Decimal
	// GrossProfit is what it made before the charges for trading were taken off.
	GrossProfit decimal.Decimal
	// TransactionCost is both charges for trading together. Funding is not among them.
	TransactionCost decimal.Decimal
	EntryTime       time.Time
	ExitTime        time.Time
}

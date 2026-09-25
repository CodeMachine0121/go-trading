package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// TradeOutcomeVo is the common shape spot and contract trades turn into so statistics are computed once.
type TradeOutcomeVo struct {
	NetProfit decimal.Decimal
	// GrossProfit is before trading charges.
	GrossProfit decimal.Decimal
	// TransactionCost is both trading charges; funding is not included.
	TransactionCost decimal.Decimal
	EntryTime       time.Time
	ExitTime        time.Time
}

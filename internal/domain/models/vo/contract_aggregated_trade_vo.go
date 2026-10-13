package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractAggregatedTradeVo is one trade from the venue's trade history, normalized from either the daily archive or the live endpoint.
type ContractAggregatedTradeVo struct {
	// Price stays a decimal so the level it falls on is decided exactly.
	Price    decimal.Decimal
	Quantity float64
	// TradeCount is how many fills the venue folded into this record; at least one.
	TradeCount int64
	TradeTime  time.Time
	// BuyerIsTaker means a buyer initiated it; otherwise a seller did.
	BuyerIsTaker bool
}

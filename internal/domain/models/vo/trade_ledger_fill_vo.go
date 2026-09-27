package vo

import (
	"time"

	"github.com/shopspring/decimal"
)

// TradeLedgerFillVo is one fill as any journal's ledger reads it; Kind is always entry or exit, whatever the journal calls it.
type TradeLedgerFillVo struct {
	ID       uint
	Kind     ContractTradeFillKindVo
	FilledAt time.Time
	Price    decimal.Decimal
	Quantity decimal.Decimal
	Fee      decimal.Decimal
}

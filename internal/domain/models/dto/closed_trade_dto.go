package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ClosedTradeDto is one round trip as it leaves the domain: which way it faced, when
// and at what price it was entered and exited, and what it made or lost.
//
// A position still open when the replay ends never becomes one of these — it has no
// exit to report, and inventing one would put a trade nobody made into the list.
type ClosedTradeDto struct {
	Direction  string          `json:"direction"`
	EntryTime  time.Time       `json:"entryTime"`
	EntryPrice decimal.Decimal `json:"entryPrice"`
	ExitTime   time.Time       `json:"exitTime"`
	ExitPrice  decimal.Decimal `json:"exitPrice"`
	// Stake is what was committed at entry, so a profit can be read against the
	// money that earned it rather than against the whole account.
	Stake decimal.Decimal `json:"stake"`
	// Profit is negative on a losing trade; there is no separate loss field.
	Profit decimal.Decimal `json:"profit"`
}

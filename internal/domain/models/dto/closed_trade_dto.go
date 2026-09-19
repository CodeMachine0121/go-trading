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
	// EntryCost and ExitCost are what this round trip paid for the act of trading, at
	// each end. Both are zero when the replay was given no rates.
	EntryCost decimal.Decimal `json:"entryCost"`
	ExitCost  decimal.Decimal `json:"exitCost"`
	// Profit is net of both charges — what the money actually did — and is negative on
	// a losing trade; there is no separate loss field. Adding the two charges back
	// gives the price move on its own.
	Profit decimal.Decimal `json:"profit"`
	// ExitReason is how the position was closed: by the signal that asked for
	// something else, or by reaching one of the two exit levels the replay was
	// given. It is on every trade rather than only in the summary's counts,
	// because a count answers how many and never which ones — and whoever reads
	// this list wants to know whether the stopped-out ones were all crowded into
	// the same stretch of market.
	ExitReason string `json:"exitReason"`
}

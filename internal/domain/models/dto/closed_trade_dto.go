package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ClosedTradeDto struct {
	Direction  string          `json:"direction"`
	EntryTime  time.Time       `json:"entryTime"`
	EntryPrice decimal.Decimal `json:"entryPrice"`
	ExitTime   time.Time       `json:"exitTime"`
	ExitPrice  decimal.Decimal `json:"exitPrice"`
	// Stake is what was committed at entry.
	Stake     decimal.Decimal `json:"stake"`
	EntryCost decimal.Decimal `json:"entryCost"`
	ExitCost  decimal.Decimal `json:"exitCost"`
	// Profit is net of both charges and negative on a loss.
	Profit decimal.Decimal `json:"profit"`
	// ExitReason is signal or one of the exit levels, kept per trade so stop-outs can be
	// located in time.
	ExitReason string `json:"exitReason"`
}

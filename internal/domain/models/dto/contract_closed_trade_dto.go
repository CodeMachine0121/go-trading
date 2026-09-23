package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// ContractClosedTradeDto is one finished round trip of a contract replay.
type ContractClosedTradeDto struct {
	Direction  string          `json:"direction"`
	EntryTime  time.Time       `json:"entryTime"`
	EntryPrice decimal.Decimal `json:"entryPrice"`
	ExitTime   time.Time       `json:"exitTime"`
	ExitPrice  decimal.Decimal `json:"exitPrice"`
	Leverage   decimal.Decimal `json:"leverage"`
	Quantity   decimal.Decimal `json:"quantity"`
	// Margin is what was taken out of the account to open this position; the notional
	// it carried is Quantity × EntryPrice.
	Margin    decimal.Decimal `json:"margin"`
	EntryCost decimal.Decimal `json:"entryCost"`
	ExitCost  decimal.Decimal `json:"exitCost"`
	// FundingFee is what this position paid in funding, net of what it received: a
	// negative figure is money it was paid.
	FundingFee decimal.Decimal `json:"fundingFee"`
	// Profit is net of both charges and of the funding.
	Profit     decimal.Decimal `json:"profit"`
	ExitReason string          `json:"exitReason"`
}

package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ContractClosedTradeDto struct {
	Direction  string          `json:"direction"`
	EntryTime  time.Time       `json:"entryTime"`
	EntryPrice decimal.Decimal `json:"entryPrice"`
	ExitTime   time.Time       `json:"exitTime"`
	ExitPrice  decimal.Decimal `json:"exitPrice"`
	Leverage   decimal.Decimal `json:"leverage"`
	Quantity   decimal.Decimal `json:"quantity"`
	// Margin is the collateral taken to open the position; notional is Quantity × EntryPrice.
	Margin    decimal.Decimal `json:"margin"`
	EntryCost decimal.Decimal `json:"entryCost"`
	ExitCost  decimal.Decimal `json:"exitCost"`
	// FundingFee is net paid; negative means it was paid funding.
	FundingFee decimal.Decimal `json:"fundingFee"`
	// Profit is net of both charges and funding.
	Profit     decimal.Decimal `json:"profit"`
	ExitReason string          `json:"exitReason"`
}

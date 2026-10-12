package dto

import "github.com/shopspring/decimal"

// ContractAutoOrderResultDto is what one round's auto order did, as shown beside the round in its history.
type ContractAutoOrderResultDto struct {
	// Status is pending, filled, partiallyDone, unfilled, notPlaced or abandoned.
	Status string `json:"status"`
	// Action is the contract act in words, such as 做多 or 反手做空; empty until something was done.
	Action            string           `json:"action,omitempty"`
	ClosedQuantity    *decimal.Decimal `json:"closedQuantity,omitempty"`
	CloseAveragePrice *decimal.Decimal `json:"closeAveragePrice,omitempty"`
	OpenedDirection   string           `json:"openedDirection,omitempty"`
	OpenedQuantity    *decimal.Decimal `json:"openedQuantity,omitempty"`
	OpenAveragePrice  *decimal.Decimal `json:"openAveragePrice,omitempty"`
	StopLossPrice     *decimal.Decimal `json:"stopLossPrice,omitempty"`
	TakeProfitPrice   *decimal.Decimal `json:"takeProfitPrice,omitempty"`
	// ReferencePrice, EntrySlippageTolerancePercentage and RequestedQuantity are what the entry order was worked out from; EntryLimitPrice is its price.
	ReferencePrice                   *decimal.Decimal `json:"referencePrice,omitempty"`
	EntrySlippageTolerancePercentage decimal.Decimal  `json:"entrySlippageTolerancePercentage"`
	RequestedQuantity                *decimal.Decimal `json:"requestedQuantity,omitempty"`
	EntryLimitPrice                  *decimal.Decimal `json:"entryLimitPrice,omitempty"`
	// EntryStatus is working, filled, partiallyFilled or unfilled; empty until an entry order is placed. EntryExecutedQuantity is what it has filled so far.
	EntryStatus           string           `json:"entryStatus,omitempty"`
	EntryExecutedQuantity *decimal.Decimal `json:"entryExecutedQuantity,omitempty"`
	// EntrySlippagePercentage is the fill against the reference price, positive when it cost the owner; present once the entry filled anything.
	EntrySlippagePercentage *decimal.Decimal `json:"entrySlippagePercentage,omitempty"`
	// ProtectionMissing is a position left open without the stop or target it was meant to have.
	ProtectionMissing bool   `json:"protectionMissing"`
	Reason            string `json:"reason,omitempty"`
}

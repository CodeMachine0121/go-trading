package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// KCandleHistorySyncRequest is the body a caller sends to have a named stretch of one
// trading symbol's history fetched now.
//
// **There is no coarseness field, and that absence is the design.** The system stores
// one kind of K candle and computes every coarser one from it; a field for it would
// suggest there is another answer, and a caller who filled it in would be told their
// choice was ignored.
//
// Unlike the catch-up next door, this one *does* let the caller say how far back —
// which is the whole of what separates them. That one asks "what am I missing" and
// the system's own setting answers it; this one asks "is that stretch here and
// correct", and only the caller knows which stretch they mean.
type KCandleHistorySyncRequest struct {
	Symbol string `json:"symbol"`
	// LookbackDays has no default. A body that leaves it out arrives as zero and is
	// refused, rather than having the one figure this route exists to be told quietly
	// filled in for it.
	LookbackDays int `json:"lookbackDays"`
}

// ToSyncDto turns the request into the shape the domain judges.
func (kCandleHistorySyncRequest KCandleHistorySyncRequest) ToSyncDto() dto.KCandleHistorySyncDto {
	return dto.KCandleHistorySyncDto{
		Symbol:       kCandleHistorySyncRequest.Symbol,
		LookbackDays: kCandleHistorySyncRequest.LookbackDays,
	}
}

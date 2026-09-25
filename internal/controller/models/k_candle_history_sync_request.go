package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// KCandleHistorySyncRequest has no interval field because only one-minute candles are stored and coarser ones are computed; unlike the catch-up, the caller picks how far back.
type KCandleHistorySyncRequest struct {
	Symbol string `json:"symbol"`
	// No default: a missing value arrives as zero and is refused.
	LookbackDays int `json:"lookbackDays"`
}

func (kCandleHistorySyncRequest KCandleHistorySyncRequest) ToSyncDto() dto.KCandleHistorySyncDto {
	return dto.KCandleHistorySyncDto{
		Symbol:       kCandleHistorySyncRequest.Symbol,
		LookbackDays: kCandleHistorySyncRequest.LookbackDays,
	}
}

package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

type WatchlistEntryRequest struct {
	Symbol string `json:"symbol"`
	Market string `json:"market"`
}

func (watchlistEntryRequest WatchlistEntryRequest) ToDto() dto.WatchlistEntryDto {
	return dto.WatchlistEntryDto{
		Symbol: watchlistEntryRequest.Symbol,
		Market: watchlistEntryRequest.Market,
	}
}

package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// WatchlistEntryRequest is the body a caller sends to start watching a market.
type WatchlistEntryRequest struct {
	Symbol string `json:"symbol"`
	Market string `json:"market"`
}

// ToDto turns the request into the shape the domain accepts. Nothing is judged here —
// which codes and which markets are acceptable is the domain's to say.
func (watchlistEntryRequest WatchlistEntryRequest) ToDto() dto.WatchlistEntryDto {
	return dto.WatchlistEntryDto{
		Symbol: watchlistEntryRequest.Symbol,
		Market: watchlistEntryRequest.Market,
	}
}

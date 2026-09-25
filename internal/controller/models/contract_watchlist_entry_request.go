package models

// ContractWatchlistEntryRequest carries only the code, since this list serves a single venue.
type ContractWatchlistEntryRequest struct {
	Symbol string `json:"symbol"`
}

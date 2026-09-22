package models

// ContractWatchlistEntryRequest is the body a caller sends to start following a
// perpetual contract.
//
// It carries only the code. Unlike the spot request beside it there is no market to
// name: this list serves exactly one venue, and a field every caller fills the same
// way only gives them a chance to fill it wrongly.
type ContractWatchlistEntryRequest struct {
	Symbol string `json:"symbol"`
}

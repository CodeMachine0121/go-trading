package models

// KCandleBackfillRequest is the body a caller sends to have one trading symbol
// caught up now.
//
// It names only the symbol. How far back to reach is the system's own setting, not
// the caller's to pick: letting a request choose would make the same button mean
// different things depending on who pressed it, and would put the market source's
// usage allowance in the hands of whoever asks loudest.
type KCandleBackfillRequest struct {
	Symbol string `json:"symbol"`
}

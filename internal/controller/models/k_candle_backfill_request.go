package models

// KCandleBackfillRequest names only the symbol; how far back to reach is a system setting so callers cannot spend the source's usage allowance at will.
type KCandleBackfillRequest struct {
	Symbol string `json:"symbol"`
}

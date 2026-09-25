package dto

// WatchlistEntryDto requires the market because the same code can mean different symbols on
// different venues and the market is never guessed.
type WatchlistEntryDto struct {
	Symbol string
	Market string
}

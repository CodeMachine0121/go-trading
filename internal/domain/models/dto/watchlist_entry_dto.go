package dto

// WatchlistEntryDto is what the application hands the domain to start watching a
// market: which market, and what it is called there.
//
// The market travels with the code because the same four digits can name different
// things in different venues, and because the system will not guess a market from
// the shape of a name — guessing is a rule that is wrong exactly once, silently.
type WatchlistEntryDto struct {
	Symbol string
	Market string
}

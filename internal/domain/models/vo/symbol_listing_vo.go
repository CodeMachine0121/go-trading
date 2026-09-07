package vo

// SymbolListingVo is what a market said when it was asked about one symbol: whether
// it lists it at all, and what it calls it.
//
// The two travel together because they come from one answer. Asking "does this
// exist" and then "what is it called" would be two round trips to the same endpoint,
// and would leave a window in which a symbol could be listed for the first question
// and not the second.
//
// DisplayName is empty for a market that names nothing — crypto pairs are already
// their own names, and inventing one would put a label on screen that the venue has
// never used.
type SymbolListingVo struct {
	IsListed    bool
	DisplayName string
}

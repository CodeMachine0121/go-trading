package vo

// SymbolListingVo is whether a market lists a symbol and its display name, from one answer; DisplayName is empty for crypto pairs.
type SymbolListingVo struct {
	IsListed    bool
	DisplayName string
}

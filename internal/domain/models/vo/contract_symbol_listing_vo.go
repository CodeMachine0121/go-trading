package vo

// ContractSymbolListingVo says whether a contract symbol can be followed and, if so, its trading specification.
type ContractSymbolListingVo struct {
	IsListed bool
	// IsKnownToVenue also holds for a perpetual the venue lists but no longer trades, which IsListed refuses.
	IsKnownToVenue bool
	Specification  ContractTradingSpecificationVo
}

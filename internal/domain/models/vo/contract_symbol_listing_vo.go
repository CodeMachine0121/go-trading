package vo

// ContractSymbolListingVo is what the contract venue said about one symbol when asked
// whether it can be followed: whether it can, and — when it can — how trading it
// looks.
type ContractSymbolListingVo struct {
	IsListed      bool
	Specification ContractTradingSpecificationVo
}

package vo

// ContractSymbolListingVo says whether a contract symbol can be followed and, if so, its trading specification.
type ContractSymbolListingVo struct {
	IsListed      bool
	Specification ContractTradingSpecificationVo
}

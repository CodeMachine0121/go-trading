package vo

// ContractOrderCallVo is the venue's answer to one call; ExchangeMessage is the venue's own words, kept so an unrecognised refusal can be passed on unchanged.
type ContractOrderCallVo struct {
	Failure         ContractOrderFailureVo
	ExchangeMessage string
}

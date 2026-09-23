package dto

// ContractTradingSymbolDto is the only shape in which a known perpetual contract
// leaves the domain.
//
// It says nothing about trading sessions or live updates, which the spot list has to:
// perpetual contracts never close, so an answer of "yes, always" on every row would
// be a column that tells the reader nothing.
type ContractTradingSymbolDto struct {
	Symbol    string `json:"symbol"`
	IsWatched bool   `json:"isWatched"`
	// TradingSpecification is null for a contract whose specification has not been
	// recorded yet, and for one known only because candles are held for it.
	TradingSpecification *ContractTradingSpecificationDto `json:"tradingSpecification"`
}

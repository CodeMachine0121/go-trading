package dto

// ContractTradingSymbolDto omits trading-session fields because perpetual contracts never close.
type ContractTradingSymbolDto struct {
	Symbol    string `json:"symbol"`
	IsWatched bool   `json:"isWatched"`
	// TradingSpecification is nil until recorded, including for contracts known only through
	// stored candles.
	TradingSpecification *ContractTradingSpecificationDto `json:"tradingSpecification"`
}

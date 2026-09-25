package dto

// TradingStrategyWriteDto is unvalidated input shared by create and rewrite; ID is zero on
// create and OwnerID comes from the signed-in user.
type TradingStrategyWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string
	// TradingMode is carried only so a request for an unsupported mode can be refused.
	TradingMode string
	// MarketDataKind blank means kCandle on create and unchanged on rewrite.
	MarketDataKind string

	SignalSources []TradingStrategySignalSourceWriteDto
	BuyCondition  TradingStrategyConditionDto
	SellCondition TradingStrategyConditionDto
}

// TradingStrategySignalSourceWriteDto Declared fields are filled by the application from the
// resolved script so the domain can validate knobs and result type up front.
type TradingStrategySignalSourceWriteDto struct {
	Label               string
	StrategyScriptID    uint
	AggregationInterval string
	ParameterValues     []StrategyScriptParameterValueDto
	DeclaredParameters  []StrategyScriptParameterWriteDto
	// DeclaredResultType must be the signal kind for a source to be usable in conditions.
	DeclaredResultType     string
	DeclaredMarketDataKind string
}

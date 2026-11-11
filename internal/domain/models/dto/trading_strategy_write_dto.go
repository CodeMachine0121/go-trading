package dto

// TradingStrategyWriteDto is unvalidated input shared by create and rewrite; ID is zero on
// create and OwnerID comes from the signed-in user.
type TradingStrategyWriteDto struct {
	ID      uint
	OwnerID uint
	Name    string
	// DeclaredTradingMode is carried only to be refused: trading strategies no longer hold one.
	DeclaredTradingMode string
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
	DeclaredResultType string
	// DeclaredUsage must be calculation; a chart script is never a signal source.
	DeclaredUsage          string
	DeclaredMarketDataKind string
}

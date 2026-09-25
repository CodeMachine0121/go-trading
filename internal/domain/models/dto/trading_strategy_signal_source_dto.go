package dto

// TradingStrategySignalSourceDto has no script field so reading a bot never exposes an
// adopted script; interval and values are per source.
type TradingStrategySignalSourceDto struct {
	// Label (A, B, C) is how conditions refer to this source.
	Label string `json:"label"`
	// StrategyScriptID is the only stored reference; no name is copied so renames never go stale.
	StrategyScriptID    uint                              `json:"strategyScriptId"`
	AggregationInterval string                            `json:"aggregationInterval"`
	ParameterValues     []StrategyScriptParameterValueDto `json:"parameterValues"`
}

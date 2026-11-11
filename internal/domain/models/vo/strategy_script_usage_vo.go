package vo

// StrategyScriptUsageVo says where a strategy script may be used; see StrategyScriptUsageDomain.
type StrategyScriptUsageVo string

const (
	// StrategyScriptUsageChart scripts are only offered on the K candle chart.
	StrategyScriptUsageChart StrategyScriptUsageVo = "chart"
	// StrategyScriptUsageCalculation scripts are the only ones trading strategies, screening and replays accept.
	StrategyScriptUsageCalculation StrategyScriptUsageVo = "calculation"
)

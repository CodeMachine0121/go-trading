package vo

// BacktestRunListFilterVo narrows one owner's runs; empty means no narrowing on that axis.
type BacktestRunListFilterVo struct {
	Status            string
	StrategyScriptID  uint
	TradingStrategyID uint
	Limit             int
}

package dto

// StrategyScriptUsageSwitchDto is one switch of a script's usage, with whatever already computes with that script.
type StrategyScriptUsageSwitchDto struct {
	ViewerID         uint
	StrategyScriptID uint
	Usage            string
	// TradingStrategyNames and ScreeningProfileNames name what computes with the script; a switch to the chart is refused while either is non-empty.
	TradingStrategyNames  []string
	ScreeningProfileNames []string
}

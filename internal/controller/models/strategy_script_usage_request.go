package models

// StrategyScriptUsageRequest is the body of a usage switch; the script comes from the path.
type StrategyScriptUsageRequest struct {
	Usage string `json:"usage"`
}

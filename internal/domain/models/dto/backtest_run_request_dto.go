package dto

// BacktestRunRequestDto is the request a run replays, kept whole so the run can be replayed later exactly as submitted; exactly one of the four is set.
type BacktestRunRequestDto struct {
	SpotScript              *BacktestRequestDto                        `json:"spotScript,omitempty"`
	ContractScript          *ContractBacktestRequestDto                `json:"contractScript,omitempty"`
	SpotTradingStrategy     *TradingStrategyBacktestRequestDto         `json:"spotTradingStrategy,omitempty"`
	ContractTradingStrategy *ContractTradingStrategyBacktestRequestDto `json:"contractTradingStrategy,omitempty"`
}

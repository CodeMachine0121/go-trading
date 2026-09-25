package vo

// TradeExitReasonVo is how one finished round trip ended, recorded per trade.
type TradeExitReasonVo string

const (
	// TradeExitReasonSignal is the only reason in a replay given no exit distances.
	TradeExitReasonSignal     TradeExitReasonVo = "signal"
	TradeExitReasonStopLoss   TradeExitReasonVo = "stopLoss"
	TradeExitReasonTakeProfit TradeExitReasonVo = "takeProfit"
	// TradeExitReasonLiquidation is the mark price reaching the maintenance margin level; contract replays only.
	TradeExitReasonLiquidation TradeExitReasonVo = "liquidation"
)

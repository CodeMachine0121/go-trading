package vo

// TradeExitReasonVo is how one finished round trip came to an end.
//
// It is recorded on every trade rather than only counted, because a total answers
// "how many" and never "which ones" — and whoever reads the trade list is asking the
// second question: were the stopped-out ones all crowded into the same stretch of
// market.
type TradeExitReasonVo string

const (
	// TradeExitReasonSignal is the only way a trade ended before exit levels existed,
	// and still the only way in a replay that was given no distances: the strategy
	// asked for something else and the position was closed to make room.
	TradeExitReasonSignal TradeExitReasonVo = "signal"
	// TradeExitReasonStopLoss is the price reaching the level set against the
	// position.
	TradeExitReasonStopLoss TradeExitReasonVo = "stopLoss"
	// TradeExitReasonTakeProfit is the price reaching the level set in its favour.
	TradeExitReasonTakeProfit TradeExitReasonVo = "takeProfit"
	// TradeExitReasonLiquidation is the price reaching the level at which the money
	// behind a borrowed position has run out, and the loan is called in. It never
	// appears in a replay that borrowed nothing.
	TradeExitReasonLiquidation TradeExitReasonVo = "liquidation"
)

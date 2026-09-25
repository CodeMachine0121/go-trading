package vo

// SignalVo is an indicator script's opinion about one candle; see SignalDomain.
type SignalVo string

const (
	// SignalBuy asks for a long: open when flat, reverse when short, nothing when already long.
	SignalBuy SignalVo = "buy"
	// SignalSell mirrors SignalBuy for a short.
	SignalSell SignalVo = "sell"
	// SignalHold leaves any position as it is and records no trade.
	SignalHold SignalVo = "hold"
)

// SignalIndicatorKey is the internal key the script runner files a signal-kind result under.
const SignalIndicatorKey = "signal"

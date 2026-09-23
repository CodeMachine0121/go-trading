package vo

// MarketDataKindVo is which kind of market a strategy script's algorithm is fed: the
// spot K candles it has always read, or the perpetual contract bars that line funding
// and positioning up beside each contract K candle. Immutable, no behavior — how a
// kind is read, defaulted and kept lives in MarketDataKindDomain.
type MarketDataKindVo string

const (
	// MarketDataKindKCandle is the spot K candle, and the kind of every strategy script
	// saved before there was a second one.
	MarketDataKindKCandle MarketDataKindVo = "kCandle"
	// MarketDataKindContractKCandle is the perpetual contract bar: a contract K candle
	// with its funding rate and position statistics lined up beside it.
	MarketDataKindContractKCandle MarketDataKindVo = "contractKCandle"
)

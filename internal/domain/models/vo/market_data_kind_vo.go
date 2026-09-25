package vo

// MarketDataKindVo is which kind of market a strategy script is fed; see MarketDataKindDomain.
type MarketDataKindVo string

const (
	// MarketDataKindKCandle is the spot K candle, the kind of every script saved before contracts existed.
	MarketDataKindKCandle MarketDataKindVo = "kCandle"
	// MarketDataKindContractKCandle is a contract K candle with its funding rate and position statistics aligned.
	MarketDataKindContractKCandle MarketDataKindVo = "contractKCandle"
)

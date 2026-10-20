package vo

// TradableMarketVo is spot or contract: the trading a Binance trading key was allowed, and the market a chart drawing board belongs to.
type TradableMarketVo string

const (
	TradableMarketSpot TradableMarketVo = "spot"
	// TradableMarketContract is USDT-margined perpetual contracts; Binance grants every futures kind with one permission.
	TradableMarketContract TradableMarketVo = "contract"
)

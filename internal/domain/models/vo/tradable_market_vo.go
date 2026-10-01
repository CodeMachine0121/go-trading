package vo

// TradableMarketVo is one kind of trading a Binance trading key was allowed when it was stored.
type TradableMarketVo string

const (
	TradableMarketSpot TradableMarketVo = "spot"
	// TradableMarketContract is USDT-margined perpetual contracts; Binance grants every futures kind with one permission.
	TradableMarketContract TradableMarketVo = "contract"
)

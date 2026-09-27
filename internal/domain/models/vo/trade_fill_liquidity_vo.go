package vo

// TradeFillLiquidityVo decides which of the person's fee rates prices a fill.
type TradeFillLiquidityVo string

const (
	TradeFillLiquidityMaker TradeFillLiquidityVo = "maker"
	TradeFillLiquidityTaker TradeFillLiquidityVo = "taker"
)

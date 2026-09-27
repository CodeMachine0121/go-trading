package vo

// SpotTradeFillKindVo has no direction: spot is bought first and sold later.
type SpotTradeFillKindVo string

const (
	SpotTradeFillKindBuy  SpotTradeFillKindVo = "buy"
	SpotTradeFillKindSell SpotTradeFillKindVo = "sell"
)

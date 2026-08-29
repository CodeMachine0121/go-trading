package vo

// KCandleVo is a K candle as an indicator script sees it: immutable plain data,
// no behavior. The open time is carried as Unix seconds rather than a time value
// so that a script can never reach the clock through it.
type KCandleVo struct {
	Symbol              string
	OpenTimeUnixSeconds int64
	Open                float64
	High                float64
	Low                 float64
	Close               float64
	Volume              float64
	QuoteVolume         float64
	TakerBuyBaseVolume  float64
	TakerBuyQuoteVolume float64
}

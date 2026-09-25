package vo

// KCandleVo is a K candle as an indicator script sees it; the open time is Unix seconds so a script can never reach the clock through it.
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

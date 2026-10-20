package vo

// ChartDrawingAnchorSnapVo names which price of the anchored candle an assistant anchor takes instead of a price of its own.
type ChartDrawingAnchorSnapVo string

const (
	ChartDrawingAnchorSnapOpen  ChartDrawingAnchorSnapVo = "open"
	ChartDrawingAnchorSnapHigh  ChartDrawingAnchorSnapVo = "high"
	ChartDrawingAnchorSnapLow   ChartDrawingAnchorSnapVo = "low"
	ChartDrawingAnchorSnapClose ChartDrawingAnchorSnapVo = "close"
)

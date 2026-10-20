package vo

// ChartDrawingSourceVo says who drew a drawing; the system records it, so a saved board can never relabel one.
type ChartDrawingSourceVo string

const (
	ChartDrawingSourceManual    ChartDrawingSourceVo = "manual"
	ChartDrawingSourceAssistant ChartDrawingSourceVo = "assistant"
)

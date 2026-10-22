package vo

// ChartDrawingKindVo is one kind of drawing on the chart workspace; its anchor counts, default style and who may draw it live in ChartDrawingKindDomain.
type ChartDrawingKindVo string

const (
	ChartDrawingKindTrendLine            ChartDrawingKindVo = "trendLine"
	ChartDrawingKindRay                  ChartDrawingKindVo = "ray"
	ChartDrawingKindHorizontalLine       ChartDrawingKindVo = "horizontalLine"
	ChartDrawingKindHorizontalRay        ChartDrawingKindVo = "horizontalRay"
	ChartDrawingKindVerticalLine         ChartDrawingKindVo = "verticalLine"
	ChartDrawingKindFibonacciRetracement ChartDrawingKindVo = "fibonacciRetracement"
	ChartDrawingKindRectangle            ChartDrawingKindVo = "rectangle"
	ChartDrawingKindDateAndPriceRange    ChartDrawingKindVo = "dateAndPriceRange"
	ChartDrawingKindBrush                ChartDrawingKindVo = "brush"
)

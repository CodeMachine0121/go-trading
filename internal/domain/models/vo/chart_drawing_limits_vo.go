package vo

// ChartDrawingLimitsVo is the one place the drawing ceilings are decided; all come from configuration.
type ChartDrawingLimitsVo struct {
	BoardCapacity          int
	BoardAnchorCapacity    int
	AssistantBatchCapacity int
}

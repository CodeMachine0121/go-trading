package vo

// ChartDrawingProvenanceVo is who drew a drawing and, for the in-app assistant, in which answer; nil identifiers mean none.
type ChartDrawingProvenanceVo struct {
	Source                  ChartDrawingSourceVo
	AssistantConversationID *uint
	AssistantTurnID         *uint
}

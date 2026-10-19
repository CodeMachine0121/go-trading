package vo

// SymbolScreeningStageVo names the gate a rejected candidate stopped at; contribution only ranks, so it is not a stage.
type SymbolScreeningStageVo string

const (
	SymbolScreeningStageNone           SymbolScreeningStageVo = ""
	SymbolScreeningStageBasicCondition SymbolScreeningStageVo = "basicCondition"
	SymbolScreeningStagePortability    SymbolScreeningStageVo = "portability"
)

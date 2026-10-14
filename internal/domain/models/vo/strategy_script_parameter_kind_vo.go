package vo

// StrategyScriptParameterKindVo is how a parameter's single numeric value is to be read.
type StrategyScriptParameterKindVo string

const (
	// StrategyScriptParameterKindLookbackCount is a positive integer of K candles to look back; the only kind the system interprets.
	StrategyScriptParameterKindLookbackCount StrategyScriptParameterKindVo = "lookbackCount"
	// StrategyScriptParameterKindNumber is any number, uninterpreted by the system.
	StrategyScriptParameterKindNumber StrategyScriptParameterKindVo = "number"
	// StrategyScriptParameterKindBoolean is stored as exactly zero or one.
	StrategyScriptParameterKindBoolean StrategyScriptParameterKindVo = "boolean"
	// StrategyScriptParameterKindOrderFlowPriceStep is a positive price width; declaring it is what asks for order flow footprints, so the system reads it like a look-back count.
	StrategyScriptParameterKindOrderFlowPriceStep StrategyScriptParameterKindVo = "orderFlowPriceStep"
)

package vo

// IndicatorResultTypeVo is the value kind a calculation's indicators carry; see IndicatorResultTypeDomain.
type IndicatorResultTypeVo string

const (
	// IndicatorResultTypeFloat is the default when a caller declares nothing.
	IndicatorResultTypeFloat     IndicatorResultTypeVo = "float"
	IndicatorResultTypeFloatList IndicatorResultTypeVo = "floatList"
	IndicatorResultTypeBool      IndicatorResultTypeVo = "bool"
	IndicatorResultTypeBoolList  IndicatorResultTypeVo = "boolList"
	// IndicatorResultTypeSignal is a single buy/sell/hold for the whole result, with no indicator name.
	IndicatorResultTypeSignal IndicatorResultTypeVo = "signal"
)

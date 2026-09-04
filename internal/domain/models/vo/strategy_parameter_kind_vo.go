package vo

// StrategyParameterKindVo is how a strategy parameter is to be read. It is a kind of
// reading, not a kind of value: every parameter's value is one number, and this says
// what that number means.
type StrategyParameterKindVo string

const (
	// StrategyParameterKindLookbackCount is a whole number greater than zero, saying
	// how many K candles this line reaches back over. It is the only kind the system
	// interprets: how many candles to read is derived from these.
	StrategyParameterKindLookbackCount StrategyParameterKindVo = "lookbackCount"
	// StrategyParameterKindNumber is any number, including negative and fractional
	// ones. The system does not read any meaning into it.
	StrategyParameterKindNumber StrategyParameterKindVo = "number"
	// StrategyParameterKindBoolean is a yes or no. It is still one number — zero is
	// no and anything else is yes — so nothing about how a value is stored or
	// supplied changes; only the reading does, which is what a kind is for.
	//
	// It is settled to exactly zero or one on the way in, so that what is stored
	// says plainly which of the two it is rather than leaving 0.7 to be interpreted
	// by whoever reads it next.
	StrategyParameterKindBoolean StrategyParameterKindVo = "boolean"
)

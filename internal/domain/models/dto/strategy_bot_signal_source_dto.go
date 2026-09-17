package dto

// StrategyBotSignalSourceDto is one strategy script as it runs inside one bot: which
// strategy script, how coarse the candles it reads are, what its knobs are worth here, and
// what it is called in the conditions.
//
// There is no script field, and that absence is the design. A bot may name a
// strategy script adopted from the marketplace, and a shape with nowhere to put a script is
// the only way "reading a bot never reads somebody else's algorithm" is something
// the types make impossible rather than something a reviewer keeps checking.
//
// The coarseness and the values live here rather than on the bot, because they are
// this source's way of running and not the bot's. One bot reading an hourly moving
// average for direction and a five-minute oscillator for timing is the ordinary
// case; a single coarseness shared by the whole bot could not say it.
type StrategyBotSignalSourceDto struct {
	// Label is what the conditions call this source — A, B, C. It is the only name
	// a condition has for a strategy script.
	Label string `json:"label"`
	// StrategyScriptID names the strategy script, and is the only thing stored about which
	// algorithm this is. There is no copy of the strategy script's name here: a copy would
	// go stale the first time it was renamed, and there is nothing it would be
	// needed for — the label is already this source's name, chosen by the person
	// who has to read it.
	StrategyScriptID    uint                              `json:"strategyScriptId"`
	AggregationInterval string                            `json:"aggregationInterval"`
	ParameterValues     []StrategyScriptParameterValueDto `json:"parameterValues"`
}

package dto

import "time"

// PublishedStrategyDto is one strategy as somebody other than its owner sees it: on
// the marketplace, or on their own shelf after adopting it.
//
// There is no Script field, and that absence is the feature. "A stranger cannot see
// the algorithm" is not a rule some code applies — it is a shape with nowhere to
// put one, so a path that forgot to redact would not compile. The alternative, one
// type with a script that is sometimes blank, needs every path to remember; this
// needs none of them to.
//
// It carries who published it, because a name and a description are not enough to
// decide whether to trust an algorithm you cannot read.
type PublishedStrategyDto struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	// Description is the only thing a reader has to judge by, the script being out
	// of reach. Empty when the owner wrote none.
	Description string `json:"description"`
	// ResultType is what this strategy produces. A reader needs it to know where the
	// values can be drawn, which is the whole point of adopting one.
	ResultType string `json:"resultType"`
	// PublisherEmail is who put it here. It is read from the strategy's owner rather
	// than stored again on the publication.
	PublisherEmail string    `json:"publisherEmail"`
	PublishedAt    time.Time `json:"publishedAt"`
	// Parameters are the knobs a reader may turn. Declaring them gives away nothing
	// about the algorithm — a knob is a name and a default, not a step.
	Parameters []StrategyParameterDto `json:"parameters"`
}

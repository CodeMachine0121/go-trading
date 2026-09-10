package dto

import "time"

// StrategyDto is the only shape a saved strategy leaves the domain in. It carries
// what the strategy remembers, never what it would compute: a strategy is a recipe,
// and its values are worked out afresh every time it is run.
//
// It carries no plan for feeding the algorithm either — how coarse the K candles
// are, how many of them, and up to when belong to one run, not to the recipe.
// It is the owner's view, and the only shape that carries the script. Everybody
// else reads PublishedStrategyDto, which has nowhere to put one — so "a stranger
// never sees the algorithm" is settled by which type is returned rather than by
// remembering to blank a field.
type StrategyDto struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	// Description is what the owner says this strategy is for. Empty when they said
	// nothing, never absent — a caller rendering it does not need two cases.
	Description string    `json:"description"`
	Script      string    `json:"script"`
	ResultType  string    `json:"resultType"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	// Published says whether this strategy is on the marketplace.
	//
	// It is only ever answered for an owner reading their own, which is the only
	// person the answer is any use to: it is what tells them whether the button in
	// front of them publishes or withdraws.
	Published bool `json:"published"`
	// Parameters are the algorithm's own knobs — the numbers it is made of, as
	// opposed to how coarse or how long, which still belong to one run.
	Parameters []StrategyParameterDto `json:"parameters"`
}

package dto

import "time"

// StrategyDto is the only shape a saved strategy leaves the domain in. It carries
// what the strategy remembers, never what it would compute: a strategy is a recipe,
// and its values are worked out afresh every time it is run.
//
// It carries no plan for feeding the algorithm either — how coarse the K candles
// are, how many of them, and up to when belong to one run, not to the recipe.
type StrategyDto struct {
	ID                  uint      `json:"id"`
	Name                string    `json:"name"`
	Script              string    `json:"script"`
	ResultType          string    `json:"resultType"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

package dto

// RunnableStrategyDto is a strategy resolved into the three things a run needs: the
// algorithm, the knobs it declares, and the kind of value it produces.
//
// It exists so that running names a strategy instead of carrying a script. That is
// the only way "usable but unreadable" is true: a script the caller sends is a
// script the caller already has, and then hiding it is only a matter of the screen
// not showing it.
//
// It travels between the domain and the application layer and stops there. No
// controller returns it and no response contains it, so resolving a strategy to run
// it never becomes a way to read it.
type RunnableStrategyDto struct {
	Script     string
	ResultType string
	// Parameters are the knobs as the strategy declares them. What they are worth
	// this time arrives with the run and is never written back — running somebody
	// else's strategy changes nothing about it.
	Parameters []StrategyParameterWriteDto
}

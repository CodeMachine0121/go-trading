package service

// contractAutoOrderStepOutcome is what one step leaves the order to do next: take the next step, wait for a retry, or nothing, since it is settled.
type contractAutoOrderStepOutcome int

const (
	stepContinues contractAutoOrderStepOutcome = iota
	stepWaits
	stepSettled
)

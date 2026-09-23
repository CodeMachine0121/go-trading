package script

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// strategyScriptParameterReader is what a script reaches a knob through.
//
// A name nobody declared is recorded and then panicked on rather than answered with
// a zero. A zero looks like an answer: a loop reaching back zero candles still
// produces a list of numbers, and somebody would act on it. Worse, a zero can turn
// a bounded loop into one that runs until the whole allowance is spent, so the
// failure arrives late as well as wrong.
//
// The panic is caught by the interpreter and comes back as an ordinary error; what
// makes the report correct is the recorded name, not the panic.
type strategyScriptParameterReader struct {
	parameters domains.StrategyScriptParametersDomain
	missing    string
	hasMissing bool
}

func (reader *strategyScriptParameterReader) lookbackCount(name string) int {
	lookbackCount, isDeclared := reader.parameters.LookbackCountOf(name)
	if !isDeclared {
		reader.recordMissing(name)
	}

	return lookbackCount
}

func (reader *strategyScriptParameterReader) number(name string) float64 {
	number, isDeclared := reader.parameters.NumberOf(name)
	if !isDeclared {
		reader.recordMissing(name)
	}

	return number
}

func (reader *strategyScriptParameterReader) boolean(name string) bool {
	isTrue, isDeclared := reader.parameters.BooleanOf(name)
	if !isDeclared {
		reader.recordMissing(name)
	}

	return isTrue
}

// recordMissing keeps the first name that did not match. The first is the one worth
// reporting: the ones after it are usually the same mistake spreading, and a list of
// them would bury the one line the reader has to go and fix.
func (reader *strategyScriptParameterReader) recordMissing(name string) {
	if !reader.hasMissing {
		reader.missing = name
		reader.hasMissing = true
	}

	panic(errParameterNotDeclared)
}

func (reader *strategyScriptParameterReader) missingName() (string, bool) {
	return reader.missing, reader.hasMissing
}

// errParameterNotDeclared is what the reader panics with. Nothing matches on it —
// the recorded name is what the report is built from — but panicking with a value of
// its own keeps this apart from a panic the script itself caused.
var errParameterNotDeclared = errors.New("indicator parameter not declared")

package script

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// strategyScriptParameterReader panics on an undeclared parameter rather than returning zero, which could yield plausible but wrong values or unbounded loops; the recorded name drives the report.
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

// recordMissing keeps only the first missing name, since later ones are usually the same mistake.
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

// errParameterNotDeclared distinguishes the reader's panic from one the script caused.
var errParameterNotDeclared = errors.New("indicator parameter not declared")

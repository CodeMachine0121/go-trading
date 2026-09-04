package domains

import (
	"errors"
	"fmt"
)

// ErrIndicatorCalculationValidation marks a request the caller got wrong:
// the candle count, or not enough candles to satisfy it.
var ErrIndicatorCalculationValidation = errors.New("indicator calculation validation failed")

// ErrIndicatorCalculationCandleCountExceeded marks the one validation failure a
// caller can act on by changing two specific inputs: the span asked for, together
// with the look-back the algorithm declares, needs more candles than one call may
// read.
//
// It is told apart from the other validation failures for the same reason the
// mismatched-knob failure is told apart from a broken script: the caller has two
// concrete ways out — ask for a shorter span, or aggregate more coarsely — and a
// caller can only offer them if it knows this is the failure it got. Reading that
// out of the sentence would be matching on prose written for a person.
var ErrIndicatorCalculationCandleCountExceeded = errors.New("indicator calculation candle count exceeded")

// CandleCountExceeded builds that failure. It answers to both sentinels: it is still
// a validation failure to everything that only cares about that, and it is the
// candle-count one to whoever can offer the two ways out.
func CandleCountExceeded(neededCandleCount int, maxCandleCount int) error {
	return &candleCountExceededError{
		neededCandleCount: neededCandleCount,
		maxCandleCount:    maxCandleCount,
	}
}

type candleCountExceededError struct {
	neededCandleCount int
	maxCandleCount    int
}

func (exceeded *candleCountExceededError) Error() string {
	return fmt.Sprintf("%v: 這一段配上回看根數要用到 %d 根，超過單次可用的最大根數（最多 %d 根）",
		ErrIndicatorCalculationValidation, exceeded.neededCandleCount, exceeded.maxCandleCount)
}

func (exceeded *candleCountExceededError) Unwrap() []error {
	return []error{ErrIndicatorCalculationValidation, ErrIndicatorCalculationCandleCountExceeded}
}

// ErrIndicatorScriptFailed marks a well-formed request whose script could not run:
// unreadable, failed while running, or reaching for something it may not use.
var ErrIndicatorScriptFailed = errors.New("indicator script failed")

// ErrIndicatorParameterNotDeclared is what a script reaching for a knob nobody
// declared comes back as.
//
// It is its own sentinel rather than a script failure, and that distinction is the
// whole point: renaming a knob and forgetting to change the line that reads it is an
// easy mistake and an invisible one, and reporting it as "your algorithm is broken"
// sends the person to read the wrong thing. What went wrong is that a name does not
// match — so that is what it says.
var ErrIndicatorParameterNotDeclared = errors.New("indicator parameter not declared")

// UndeclaredParameterName digs the name out of a mismatched-knob failure, so that
// whoever answers the caller can hand it over as a field of its own.
//
// A caller telling this failure apart by reading the message would be matching on
// prose written for a person — it changes whenever the wording improves. The name
// travels as a value instead.
func UndeclaredParameterName(err error) (string, bool) {
	var undeclared *undeclaredParameterError
	if !errors.As(err, &undeclared) {
		return "", false
	}

	return undeclared.name, true
}

// UndeclaredParameter builds the failure for a knob nobody declared, carrying both
// the sentence a person reads and the name a caller acts on.
func UndeclaredParameter(name string) error {
	return &undeclaredParameterError{name: name}
}

type undeclaredParameterError struct {
	name string
}

func (undeclared *undeclaredParameterError) Error() string {
	return fmt.Sprintf("%v: 算式取用了參數 %q，但這一次沒有宣告這個名字",
		ErrIndicatorParameterNotDeclared, undeclared.name)
}

func (undeclared *undeclaredParameterError) Unwrap() error {
	return ErrIndicatorParameterNotDeclared
}

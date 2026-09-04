package domains

import "errors"

// ErrIndicatorCalculationValidation marks a request the caller got wrong:
// the candle count, or not enough candles to satisfy it.
var ErrIndicatorCalculationValidation = errors.New("indicator calculation validation failed")

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

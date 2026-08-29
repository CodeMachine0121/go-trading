package domains

import "errors"

// ErrIndicatorCalculationValidation marks a request the caller got wrong:
// the candle count, or not enough candles to satisfy it.
var ErrIndicatorCalculationValidation = errors.New("indicator calculation validation failed")

// ErrIndicatorScriptFailed marks a well-formed request whose script could not run:
// unreadable, failed while running, or reaching for something it may not use.
var ErrIndicatorScriptFailed = errors.New("indicator script failed")

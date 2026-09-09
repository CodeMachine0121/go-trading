package domains

import "errors"

// ErrObservationWindowValidation marks a stretch of market nobody could look at: no
// beginning was named, or it begins after it ends.
//
// It is the window's own sentinel rather than any one caller's, because the window
// outlives the first thing that asked for it — an indicator calculation today, a
// backtest tomorrow — and each of those wraps it in whatever its own callers already
// recognise.
var ErrObservationWindowValidation = errors.New("observation window validation failed")

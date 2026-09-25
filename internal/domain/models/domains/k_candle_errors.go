package domains

import "errors"

// ErrKCandleValidation marks any failed K candle rule; the wrapped message names the rule.
var ErrKCandleValidation = errors.New("k candle validation failed")

var ErrKCandleNotFound = errors.New("k candle not found")

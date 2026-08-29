package domains

import "errors"

// ErrKCandleValidation marks any K candle rule that did not pass. The wrapped
// message names the rule so the caller can report it without knowing the rule list.
var ErrKCandleValidation = errors.New("k candle validation failed")

// ErrKCandleNotFound marks a K candle named by symbol and open time that does not exist.
var ErrKCandleNotFound = errors.New("k candle not found")

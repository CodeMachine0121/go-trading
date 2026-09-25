package domains

import "errors"

// ErrKCandleContractValidation marks any failed contract K candle rule; the wrapped message names the rule.
var ErrKCandleContractValidation = errors.New("contract k candle validation failed")

var ErrKCandleContractNotFound = errors.New("contract k candle not found")

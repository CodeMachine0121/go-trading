package domains

import "errors"

// ErrKCandleContractValidation marks any contract K candle rule that did not pass.
// The wrapped message names the rule so the caller can report it without knowing the
// rule list — the same arrangement the spot K candle uses.
var ErrKCandleContractValidation = errors.New("contract k candle validation failed")

// ErrKCandleContractNotFound marks a contract K candle named by symbol and open time
// that does not exist.
var ErrKCandleContractNotFound = errors.New("contract k candle not found")

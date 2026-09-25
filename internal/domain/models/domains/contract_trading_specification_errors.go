package domains

import "errors"

// ErrContractTradingSpecificationValidation marks an impossible venue-reported specification; the wrapped message names the rule.
var ErrContractTradingSpecificationValidation = errors.New("contract trading specification validation failed")

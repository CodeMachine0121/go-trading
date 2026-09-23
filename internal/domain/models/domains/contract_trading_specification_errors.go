package domains

import "errors"

// ErrContractTradingSpecificationValidation marks a trading specification the venue
// reported that cannot be one. The wrapped message names the rule.
var ErrContractTradingSpecificationValidation = errors.New("contract trading specification validation failed")

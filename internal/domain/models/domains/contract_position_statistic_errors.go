package domains

import "errors"

// ErrContractPositionStatisticValidation marks any failed position statistic or query rule; the wrapped message names the rule.
var ErrContractPositionStatisticValidation = errors.New("contract position statistic validation failed")

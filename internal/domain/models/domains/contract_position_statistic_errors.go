package domains

import "errors"

// ErrContractPositionStatisticValidation marks any position statistic rule — or
// position statistic query rule — that did not pass. The wrapped message names the
// rule.
var ErrContractPositionStatisticValidation = errors.New("contract position statistic validation failed")

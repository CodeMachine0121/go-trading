package domains

import "errors"

// ErrContractFundingRateSettlementValidation marks any funding rate settlement rule —
// or funding rate settlement query rule — that did not pass. The wrapped message
// names the rule.
var ErrContractFundingRateSettlementValidation = errors.New("contract funding rate settlement validation failed")

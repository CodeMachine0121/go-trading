package domains

import "errors"

// ErrContractFundingRateSettlementValidation marks a failed settlement or settlement query rule; the wrapped message names it.
var ErrContractFundingRateSettlementValidation = errors.New("contract funding rate settlement validation failed")

package domains

import "errors"

// ErrContractMaintenanceMarginTierValidation marks a failed ladder or ladder query rule; the wrapped message names it.
var ErrContractMaintenanceMarginTierValidation = errors.New("contract maintenance margin tier validation failed")

package domains

import "errors"

// ErrContractMaintenanceMarginTierValidation marks a maintenance margin ladder — or a
// query for one — that did not pass a rule. The wrapped message names the rule.
var ErrContractMaintenanceMarginTierValidation = errors.New("contract maintenance margin tier validation failed")

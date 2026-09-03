package domains

import "errors"

// ErrStrategyValidation marks a strategy whose content broke one of its rules. The
// wrapped message names the rule, so a caller reports it without knowing the list.
var ErrStrategyValidation = errors.New("strategy validation failed")

// ErrStrategyNameConflict marks a strategy name another strategy already holds. It
// is deliberately not a validation failure: the content is fine, the name is taken,
// and the two lead a caller to do different things about it.
var ErrStrategyNameConflict = errors.New("strategy name already in use")

// ErrStrategyNotFound marks a strategy named by an identifier that does not exist.
var ErrStrategyNotFound = errors.New("strategy not found")

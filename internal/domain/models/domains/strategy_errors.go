package domains

import (
	"errors"
	"fmt"
)

// ErrStrategyValidation marks a strategy whose content broke one of its rules. The
// wrapped message names the rule, so a caller reports it without knowing the list.
var ErrStrategyValidation = errors.New("strategy validation failed")

// ErrStrategyNameConflict marks a strategy name another strategy already holds. It
// is deliberately not a validation failure: the content is fine, the name is taken,
// and the two lead a caller to do different things about it.
var ErrStrategyNameConflict = errors.New("strategy name already in use")

// ErrStrategyNotFound marks a strategy named by an identifier that does not exist.
var ErrStrategyNotFound = errors.New("strategy not found")

// StrategyNotFound is the refusal a caller reads when no strategy carries this
// identifier. Two places arrive at it — the store, which looked and found nothing,
// and the service, which knows before looking that no strategy carries no
// identifier — and both owe the reader the same sentence. Worded twice it was
// worded differently, and the one refusal that came out in the system's own
// language read as though it had come from somewhere else entirely.
func StrategyNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的策略", ErrStrategyNotFound, id)
}

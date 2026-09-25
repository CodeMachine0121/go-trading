package domains

import (
	"errors"
	"fmt"
)

// ErrStrategyScriptValidation wraps a message naming the broken rule.
var ErrStrategyScriptValidation = errors.New("strategy script validation failed")

// ErrStrategyScriptNameConflict is deliberately not a validation failure: the content is fine but the name is taken.
var ErrStrategyScriptNameConflict = errors.New("strategy script name already in use")

var ErrStrategyScriptNotFound = errors.New("strategy script not found")

// StrategyScriptNotFound is the shared wording for both the store and the service so the refusal reads identically.
func StrategyScriptNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的策略腳本", ErrStrategyScriptNotFound, id)
}

// ErrStrategyScriptNotPublished is internal only: callers convert it to not-found so a private script's existence isn't revealed.
var ErrStrategyScriptNotPublished = errors.New("strategy script not published")

// ErrRunSubjectAmbiguous marks a run that both named a strategy script and supplied an algorithm, or neither.
var ErrRunSubjectAmbiguous = errors.New("run subject ambiguous")

// ErrStrategyScriptMarketDataKindMismatch marks a well-formed script written for the other market data kind.
var ErrStrategyScriptMarketDataKindMismatch = errors.New("strategy script market data kind mismatch")

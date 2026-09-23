package domains

import (
	"errors"
	"fmt"
)

// ErrStrategyScriptValidation marks a strategy script whose content broke one of its rules. The
// wrapped message names the rule, so a caller reports it without knowing the list.
var ErrStrategyScriptValidation = errors.New("strategy script validation failed")

// ErrStrategyScriptNameConflict marks a strategy script name another strategy script already holds. It
// is deliberately not a validation failure: the content is fine, the name is taken,
// and the two lead a caller to do different things about it.
var ErrStrategyScriptNameConflict = errors.New("strategy script name already in use")

// ErrStrategyScriptNotFound marks a strategy script named by an identifier that does not exist.
var ErrStrategyScriptNotFound = errors.New("strategy script not found")

// StrategyScriptNotFound is the refusal a caller reads when no strategy script carries this
// identifier. Two places arrive at it — the store, which looked and found nothing,
// and the service, which knows before looking that no strategy script carries no
// identifier — and both owe the reader the same sentence. Worded twice it was
// worded differently, and the one refusal that came out in the system's own
// language read as though it had come from somewhere else entirely.
func StrategyScriptNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的策略腳本", ErrStrategyScriptNotFound, id)
}

// ErrStrategyScriptNotPublished marks a strategy script that is not on the marketplace. It never
// reaches a caller: every path that meets it turns it into the one refusal a closed
// door gives, because telling somebody "that exists but is not shared" is telling
// them it exists. It is only how the store says "there is no publication here" to
// the code above it — the same role ErrUserNotFound plays for a sign-in.
var ErrStrategyScriptNotPublished = errors.New("strategy script not published")

// ErrRunSubjectAmbiguous marks a run that named a strategy script and carried an algorithm
// at the same time, or did neither.
//
// The two are alternatives, not a pair: naming a strategy script fetches an algorithm, and
// carrying one supplies it. Sent together they can disagree, and the system would
// have to pick — a decision nobody asked it to make. Sent neither way there is
// nothing to run.
var ErrRunSubjectAmbiguous = errors.New("run subject ambiguous")

// ErrStrategyScriptMarketDataKindMismatch marks a strategy script named for a
// calculation that feeds the other kind of market. It is not a validation failure of
// the request, and not a script failure either: the request is well formed and the
// script is written right — for a different market.
var ErrStrategyScriptMarketDataKindMismatch = errors.New("strategy script market data kind mismatch")

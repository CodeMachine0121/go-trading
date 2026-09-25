package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ErrForeignStrategyScriptFailed marks a script failure whose wording came out of someone else's script, so
// only a person, never the assistant, may read it.
var ErrForeignStrategyScriptFailed = errors.New("foreign strategy script failed")

// StrategyScriptAuthorshipDomain says whose words a run's failure can carry: a run counts as the viewer's only
// when every script in it is, since a failed replay cannot say which of its sources failed.
type StrategyScriptAuthorshipDomain struct {
	ownedByViewer bool
}

func NewStrategyScriptAuthorshipDomain(
	runnableStrategyScripts []dto.RunnableStrategyScriptDto,
) StrategyScriptAuthorshipDomain {
	ownedByViewer := true
	for _, runnableStrategyScript := range runnableStrategyScripts {
		ownedByViewer = ownedByViewer && runnableStrategyScript.OwnedByViewer
	}

	return StrategyScriptAuthorshipDomain{ownedByViewer: ownedByViewer}
}

// AttributeFailure marks every failure that came out of the script compartment, a timeout included, because
// the compartment hands back one message and cannot vouch that none of it was written by the script.
// Refusals the system makes before the script runs are left alone.
func (strategyScriptAuthorshipDomain StrategyScriptAuthorshipDomain) AttributeFailure(runError error) error {
	if runError == nil || strategyScriptAuthorshipDomain.ownedByViewer {
		return runError
	}

	if !errors.Is(runError, ErrIndicatorScriptFailed) && !errors.Is(runError, ErrIndicatorParameterNotDeclared) {
		return runError
	}

	return &foreignStrategyScriptFailureError{cause: runError}
}

// foreignStrategyScriptFailureError reads exactly like its cause, so people see the same message as before.
type foreignStrategyScriptFailureError struct {
	cause error
}

func (foreignFailure *foreignStrategyScriptFailureError) Error() string {
	return foreignFailure.cause.Error()
}

func (foreignFailure *foreignStrategyScriptFailureError) Unwrap() []error {
	return []error{foreignFailure.cause, ErrForeignStrategyScriptFailed}
}

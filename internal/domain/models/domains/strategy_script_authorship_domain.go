package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

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
		ownedByViewer = ownedByViewer && runnableStrategyScript.AuthoredByViewer
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

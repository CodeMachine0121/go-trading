package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_assistant_pending_revision_repository.go -destination=mocks/mock_i_assistant_pending_revision_repository.go -package=mocks

// IAssistantPendingRevisionRepository stores proposals as they are made; conversations read them back with their turns.
type IAssistantPendingRevisionRepository interface {
	Save(
		executionContext context.Context, assistantPendingRevision entities.AssistantPendingRevision,
	) (entities.AssistantPendingRevision, error)
	// FindOne maps storage not-found to ErrAssistantPendingRevisionNotFound.
	FindOne(executionContext context.Context, id uint) (entities.AssistantPendingRevision, error)
	// TransitionStatus moves the proposal only if it is still in the from status, and reports whether it did,
	// so two presses at once can never both act.
	TransitionStatus(executionContext context.Context, id uint, from string, to string) (bool, error)
}

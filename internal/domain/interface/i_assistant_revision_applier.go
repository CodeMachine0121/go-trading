package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_assistant_revision_applier.go -destination=mocks/mock_i_assistant_revision_applier.go -package=mocks

// IAssistantRevisionApplier is one kind of thing the assistant can rewrite; offering it another kind means adding an
// applier, not changing how proposals are made, confirmed or rejected.
type IAssistantRevisionApplier interface {
	SubjectKind() vo.AssistantRevisionSubjectKindVo
	// Inspect checks the content under the ordinary rewrite rules as the viewer, writing nothing, and returns the
	// subject as it stands now.
	Inspect(executionContext context.Context, viewerID uint, content string) (dto.RewriteTargetDto, error)
	// Apply performs the ordinary rewrite and returns what the assistant should read of the result.
	Apply(executionContext context.Context, viewerID uint, content string) (string, error)
}

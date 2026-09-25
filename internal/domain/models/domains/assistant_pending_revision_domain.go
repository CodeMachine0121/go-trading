package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantPendingRevisionDomain holds the rules for acting on a proposal: only its owner may, only while pending,
// and only while its subject is still the version it was proposed against.
type AssistantPendingRevisionDomain struct {
	assistantPendingRevision entities.AssistantPendingRevision
}

func NewAssistantPendingRevisionDomain(
	assistantPendingRevision entities.AssistantPendingRevision,
) AssistantPendingRevisionDomain {
	return AssistantPendingRevisionDomain{assistantPendingRevision: assistantPendingRevision}
}

// RequireActionableBy checks ownership before state, so probing someone else's proposal reads as not found rather than handled.
func (assistantPendingRevisionDomain AssistantPendingRevisionDomain) RequireActionableBy(viewerID uint) error {
	if viewerID == 0 || assistantPendingRevisionDomain.assistantPendingRevision.OwnerID != viewerID {
		return AssistantPendingRevisionNotFound(assistantPendingRevisionDomain.assistantPendingRevision.ID)
	}

	if vo.AssistantPendingRevisionStatusVo(assistantPendingRevisionDomain.assistantPendingRevision.Status) !=
		vo.AssistantPendingRevisionPending {
		return AssistantPendingRevisionResolved()
	}

	return nil
}

// RequireUnchangedSince compares instants, not representations, since the store may hand back another location.
func (assistantPendingRevisionDomain AssistantPendingRevisionDomain) RequireUnchangedSince(
	subjectUpdatedAt time.Time,
) error {
	if !assistantPendingRevisionDomain.assistantPendingRevision.SubjectUpdatedAt.Equal(subjectUpdatedAt) {
		return AssistantPendingRevisionStale()
	}

	return nil
}

func (assistantPendingRevisionDomain AssistantPendingRevisionDomain) ToDto() dto.AssistantPendingRevisionDto {
	return assistantPendingRevisionDomain.assistantPendingRevision.ToDto()
}

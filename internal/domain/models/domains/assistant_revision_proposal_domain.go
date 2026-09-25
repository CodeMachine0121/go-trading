package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantRevisionProposalDomain decides whether the assistant's rewrite is written now or left for the owner:
// only what the assistant created in this very conversation, and no bot uses, is written now. Anything else could
// be the owner's own work or feed a bot's signals, and an assistant misled by text it read must not change those.
type AssistantRevisionProposalDomain struct {
	proposal              dto.AssistantRevisionProposalDto
	createdInConversation bool
	proposedAt            time.Time
}

func NewAssistantRevisionProposalDomain(
	proposal dto.AssistantRevisionProposalDto, createdInConversation bool, proposedAt time.Time,
) AssistantRevisionProposalDomain {
	return AssistantRevisionProposalDomain{
		proposal:              proposal,
		createdInConversation: createdInConversation,
		proposedAt:            proposedAt,
	}
}

func (assistantRevisionProposalDomain AssistantRevisionProposalDomain) AppliesDirectly() bool {
	return assistantRevisionProposalDomain.createdInConversation &&
		assistantRevisionProposalDomain.proposal.Target.BotReferenceCount == 0
}

func (assistantRevisionProposalDomain AssistantRevisionProposalDomain) ToEntity() entities.AssistantPendingRevision {
	proposal := assistantRevisionProposalDomain.proposal

	return entities.AssistantPendingRevision{
		OwnerID:          proposal.ViewerID,
		ConversationID:   proposal.ConversationID,
		AssistantTurnID:  proposal.TurnID,
		SubjectKind:      proposal.SubjectKind,
		SubjectID:        proposal.Target.ID,
		SubjectName:      proposal.Target.Name,
		Content:          proposal.Content,
		SubjectUpdatedAt: proposal.Target.UpdatedAt.UTC(),
		Status:           string(vo.AssistantPendingRevisionPending),
		ProposedAt:       assistantRevisionProposalDomain.proposedAt.UTC(),
	}
}

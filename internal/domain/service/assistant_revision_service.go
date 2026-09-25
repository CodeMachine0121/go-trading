package service

import (
	"context"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantRevisionService keeps the assistant's proposed rewrites and what it created in each conversation; it never
// writes the rewrites themselves, which the application layer hands to the owning use case.
type AssistantRevisionService struct {
	pendingRevisionRepository domaininterface.IAssistantPendingRevisionRepository
	createdSubjectRepository  domaininterface.IAssistantCreatedSubjectRepository
	clockProxy                domaininterface.IClockProxy
}

func NewAssistantRevisionService(
	pendingRevisionRepository domaininterface.IAssistantPendingRevisionRepository,
	createdSubjectRepository domaininterface.IAssistantCreatedSubjectRepository,
	clockProxy domaininterface.IClockProxy,
) *AssistantRevisionService {
	return &AssistantRevisionService{
		pendingRevisionRepository: pendingRevisionRepository,
		createdSubjectRepository:  createdSubjectRepository,
		clockProxy:                clockProxy,
	}
}

// Propose leaves the rewrite for the owner unless it may be written now, in which case nothing is stored.
func (assistantRevisionService *AssistantRevisionService) Propose(
	executionContext context.Context, proposal dto.AssistantRevisionProposalDto,
) (dto.AssistantRevisionProposalOutcomeDto, error) {
	createdInConversation, existsError := assistantRevisionService.createdSubjectRepository.Exists(
		executionContext, proposal.ConversationID, proposal.SubjectKind, proposal.Target.ID)
	if existsError != nil {
		return dto.AssistantRevisionProposalOutcomeDto{}, existsError
	}

	proposalDomain := domains.NewAssistantRevisionProposalDomain(
		proposal, createdInConversation, assistantRevisionService.clockProxy.Now())
	if proposalDomain.AppliesDirectly() {
		return dto.AssistantRevisionProposalOutcomeDto{AppliesDirectly: true}, nil
	}

	savedRevision, saveError := assistantRevisionService.pendingRevisionRepository.Save(
		executionContext, proposalDomain.ToEntity())
	if saveError != nil {
		return dto.AssistantRevisionProposalOutcomeDto{}, saveError
	}

	return dto.AssistantRevisionProposalOutcomeDto{PendingRevision: savedRevision.ToDto()}, nil
}

func (assistantRevisionService *AssistantRevisionService) RecordCreatedSubject(
	executionContext context.Context,
	origin vo.AssistantQueryOriginVo,
	subjectKind vo.AssistantRevisionSubjectKindVo,
	subjectID uint,
) error {
	return assistantRevisionService.createdSubjectRepository.Save(executionContext, entities.AssistantCreatedSubject{
		ConversationID: origin.ConversationID,
		SubjectKind:    string(subjectKind),
		SubjectID:      subjectID,
		CreatedAt:      assistantRevisionService.clockProxy.Now().UTC(),
	})
}

// FindPendingRevision returns a proposal its owner may still act on.
func (assistantRevisionService *AssistantRevisionService) FindPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	revisionDomain, findError := assistantRevisionService.findActionable(executionContext, viewerID, id)
	if findError != nil {
		return dto.AssistantPendingRevisionDto{}, findError
	}

	return revisionDomain.ToDto(), nil
}

// ClaimConfirmation marks the proposal confirmed before it is written, so a second press cannot write it again;
// the caller reopens it if the write is then refused.
func (assistantRevisionService *AssistantRevisionService) ClaimConfirmation(
	executionContext context.Context, viewerID uint, id uint, subjectUpdatedAt time.Time,
) (dto.AssistantPendingRevisionDto, error) {
	revisionDomain, findError := assistantRevisionService.findActionable(executionContext, viewerID, id)
	if findError != nil {
		return dto.AssistantPendingRevisionDto{}, findError
	}

	if staleError := revisionDomain.RequireUnchangedSince(subjectUpdatedAt); staleError != nil {
		return dto.AssistantPendingRevisionDto{}, staleError
	}

	return assistantRevisionService.transition(
		executionContext, revisionDomain, vo.AssistantPendingRevisionConfirmed)
}

// ReopenPendingRevision undoes a claim whose write was refused, so the owner can deal with the reason and press again.
func (assistantRevisionService *AssistantRevisionService) ReopenPendingRevision(
	executionContext context.Context, id uint,
) error {
	_, transitionError := assistantRevisionService.pendingRevisionRepository.TransitionStatus(
		executionContext, id,
		string(vo.AssistantPendingRevisionConfirmed), string(vo.AssistantPendingRevisionPending))

	return transitionError
}

func (assistantRevisionService *AssistantRevisionService) RejectPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	revisionDomain, findError := assistantRevisionService.findActionable(executionContext, viewerID, id)
	if findError != nil {
		return dto.AssistantPendingRevisionDto{}, findError
	}

	return assistantRevisionService.transition(
		executionContext, revisionDomain, vo.AssistantPendingRevisionRejected)
}

func (assistantRevisionService *AssistantRevisionService) findActionable(
	executionContext context.Context, viewerID uint, id uint,
) (domains.AssistantPendingRevisionDomain, error) {
	assistantPendingRevision, findError := assistantRevisionService.pendingRevisionRepository.FindOne(
		executionContext, id)
	if findError != nil {
		return domains.AssistantPendingRevisionDomain{}, findError
	}

	revisionDomain := domains.NewAssistantPendingRevisionDomain(assistantPendingRevision)

	return revisionDomain, revisionDomain.RequireActionableBy(viewerID)
}

// transition loses a race to a simultaneous press as "already handled", the same answer the press would get a moment later.
func (assistantRevisionService *AssistantRevisionService) transition(
	executionContext context.Context,
	revisionDomain domains.AssistantPendingRevisionDomain,
	to vo.AssistantPendingRevisionStatusVo,
) (dto.AssistantPendingRevisionDto, error) {
	revisionDto := revisionDomain.ToDto()

	transitioned, transitionError := assistantRevisionService.pendingRevisionRepository.TransitionStatus(
		executionContext, revisionDto.ID, string(vo.AssistantPendingRevisionPending), string(to))
	if transitionError != nil {
		return dto.AssistantPendingRevisionDto{}, transitionError
	}
	if !transitioned {
		return dto.AssistantPendingRevisionDto{}, domains.AssistantPendingRevisionResolved()
	}

	revisionDto.Status = string(to)

	return revisionDto, nil
}

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// AssistantRevisionService turns the assistant's rewrites of what a person already has into proposals the person
// confirms, and carries a confirmed one out through the applier of its kind; each kind's rewrite rules stay with it.
type AssistantRevisionService struct {
	pendingRevisionRepository domaininterface.IAssistantPendingRevisionRepository
	createdSubjectRepository  domaininterface.IAssistantCreatedSubjectRepository
	appliers                  []domaininterface.IAssistantRevisionApplier
	clockProxy                domaininterface.IClockProxy
}

func NewAssistantRevisionService(
	pendingRevisionRepository domaininterface.IAssistantPendingRevisionRepository,
	createdSubjectRepository domaininterface.IAssistantCreatedSubjectRepository,
	appliers []domaininterface.IAssistantRevisionApplier,
	clockProxy domaininterface.IClockProxy,
) *AssistantRevisionService {
	return &AssistantRevisionService{
		pendingRevisionRepository: pendingRevisionRepository,
		createdSubjectRepository:  createdSubjectRepository,
		appliers:                  appliers,
		clockProxy:                clockProxy,
	}
}

// Revise checks the rewrite first, so one that could never be carried out is refused to the assistant rather than
// left for the owner; it then writes it now or leaves it pending, and returns what the assistant should read.
func (assistantRevisionService *AssistantRevisionService) Revise(
	executionContext context.Context,
	origin vo.AssistantQueryOriginVo,
	subjectKind vo.AssistantRevisionSubjectKindVo,
	content string,
) (string, error) {
	applier, applierError := assistantRevisionService.applierFor(string(subjectKind))
	if applierError != nil {
		return "", applierError
	}

	target, inspectError := applier.Inspect(executionContext, origin.ViewerID, content)
	if inspectError != nil {
		return "", inspectError
	}

	createdInConversation, existsError := assistantRevisionService.createdSubjectRepository.Exists(
		executionContext, origin.ConversationID, string(subjectKind), target.ID)
	if existsError != nil {
		return "", existsError
	}

	proposalDomain := domains.NewAssistantRevisionProposalDomain(dto.AssistantRevisionProposalDto{
		ViewerID:       origin.ViewerID,
		ConversationID: origin.ConversationID,
		TurnID:         origin.TurnID,
		SubjectKind:    string(subjectKind),
		Target:         target,
		Content:        content,
	}, createdInConversation, assistantRevisionService.clockProxy.Now())
	if proposalDomain.AppliesDirectly() {
		return applier.Apply(executionContext, origin.ViewerID, content)
	}

	savedRevision, saveError := assistantRevisionService.pendingRevisionRepository.Save(
		executionContext, proposalDomain.ToEntity())
	if saveError != nil {
		return "", saveError
	}

	payload, marshalError := json.Marshal(dto.AssistantPendingRevisionReportDto{
		PendingRevisionID: savedRevision.ID,
		Status:            savedRevision.Status,
		Notice:            domains.AssistantRevisionProposedNotice,
	})
	if marshalError != nil {
		return "", fmt.Errorf("render pending revision: %w", marshalError)
	}

	return string(payload), nil
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

// ConfirmPendingRevision claims the proposal before writing, so it is written at most once, and hands it back to
// pending when the ordinary rewrite refuses it, so the owner can deal with the reason and confirm again.
func (assistantRevisionService *AssistantRevisionService) ConfirmPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	revisionDomain, findError := assistantRevisionService.findActionable(executionContext, viewerID, id)
	if findError != nil {
		return dto.AssistantPendingRevisionDto{}, findError
	}

	pendingRevision := revisionDomain.ToDto()
	content := string(pendingRevision.Content)

	applier, applierError := assistantRevisionService.applierFor(pendingRevision.SubjectKind)
	if applierError != nil {
		return dto.AssistantPendingRevisionDto{}, applierError
	}

	target, inspectError := applier.Inspect(executionContext, viewerID, content)
	if inspectError != nil {
		return dto.AssistantPendingRevisionDto{}, inspectError
	}

	if staleError := revisionDomain.RequireUnchangedSince(target.UpdatedAt); staleError != nil {
		return dto.AssistantPendingRevisionDto{}, staleError
	}

	confirmedRevision, claimError := assistantRevisionService.transition(
		executionContext, pendingRevision, vo.AssistantPendingRevisionPending, vo.AssistantPendingRevisionConfirmed)
	if claimError != nil {
		return dto.AssistantPendingRevisionDto{}, claimError
	}

	if _, applyError := applier.Apply(executionContext, viewerID, content); applyError != nil {
		// Detached from the request, since a press that timed out or was abandoned must still hand the revision back.
		_, reopenError := assistantRevisionService.transition(
			context.WithoutCancel(executionContext), confirmedRevision,
			vo.AssistantPendingRevisionConfirmed, vo.AssistantPendingRevisionPending)

		return dto.AssistantPendingRevisionDto{}, errors.Join(applyError, reopenError)
	}

	return confirmedRevision, nil
}

func (assistantRevisionService *AssistantRevisionService) RejectPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	revisionDomain, findError := assistantRevisionService.findActionable(executionContext, viewerID, id)
	if findError != nil {
		return dto.AssistantPendingRevisionDto{}, findError
	}

	return assistantRevisionService.transition(
		executionContext, revisionDomain.ToDto(),
		vo.AssistantPendingRevisionPending, vo.AssistantPendingRevisionRejected)
}

func (assistantRevisionService *AssistantRevisionService) applierFor(
	subjectKind string,
) (domaininterface.IAssistantRevisionApplier, error) {
	for _, applier := range assistantRevisionService.appliers {
		if string(applier.SubjectKind()) == subjectKind {
			return applier, nil
		}
	}

	return nil, fmt.Errorf("no assistant revision applier for %q", subjectKind)
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
	revision dto.AssistantPendingRevisionDto,
	from vo.AssistantPendingRevisionStatusVo,
	to vo.AssistantPendingRevisionStatusVo,
) (dto.AssistantPendingRevisionDto, error) {
	transitioned, transitionError := assistantRevisionService.pendingRevisionRepository.TransitionStatus(
		executionContext, revision.ID, string(from), string(to))
	if transitionError != nil {
		return dto.AssistantPendingRevisionDto{}, transitionError
	}
	if !transitioned {
		return dto.AssistantPendingRevisionDto{}, domains.AssistantPendingRevisionResolved()
	}

	revision.Status = string(to)

	return revision, nil
}

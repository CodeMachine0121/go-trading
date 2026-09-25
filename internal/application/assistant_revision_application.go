package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// AssistantRevisionApplication turns the assistant's rewrites of what a person already has into proposals the
// person confirms, and carries a confirmed one out through the ordinary rewrite of its kind.
type AssistantRevisionApplication struct {
	assistantRevisionService *service.AssistantRevisionService
	appliers                 []domaininterface.IAssistantRevisionApplier
}

func NewAssistantRevisionApplication(
	assistantRevisionService *service.AssistantRevisionService,
	appliers []domaininterface.IAssistantRevisionApplier,
) *AssistantRevisionApplication {
	return &AssistantRevisionApplication{
		assistantRevisionService: assistantRevisionService,
		appliers:                 appliers,
	}
}

// pendingRevisionReport is what the assistant reads when its rewrite waits for the owner.
type pendingRevisionReport struct {
	PendingRevisionID uint   `json:"pendingRevisionId"`
	Status            string `json:"status"`
	Notice            string `json:"notice"`
}

// Revise checks the rewrite first, so a rewrite that could never be carried out is refused to the assistant rather
// than left for the owner; it then writes it now or leaves it pending, and says which to the assistant.
func (assistantRevisionApplication *AssistantRevisionApplication) Revise(
	executionContext context.Context,
	origin vo.AssistantQueryOriginVo,
	subjectKind vo.AssistantRevisionSubjectKindVo,
	content string,
) (string, error) {
	applier, applierError := assistantRevisionApplication.applierFor(string(subjectKind))
	if applierError != nil {
		return "", applierError
	}

	target, inspectError := applier.Inspect(executionContext, origin.ViewerID, content)
	if inspectError != nil {
		return "", inspectError
	}

	outcome, proposeError := assistantRevisionApplication.assistantRevisionService.Propose(
		executionContext, dto.AssistantRevisionProposalDto{
			ViewerID:       origin.ViewerID,
			ConversationID: origin.ConversationID,
			TurnID:         origin.TurnID,
			SubjectKind:    string(subjectKind),
			Target:         target,
			Content:        content,
		})
	if proposeError != nil {
		return "", proposeError
	}

	if outcome.AppliesDirectly {
		return applier.Apply(executionContext, origin.ViewerID, content)
	}

	payload, marshalError := json.Marshal(pendingRevisionReport{
		PendingRevisionID: outcome.PendingRevision.ID,
		Status:            outcome.PendingRevision.Status,
		Notice:            domains.AssistantRevisionProposedNotice,
	})
	if marshalError != nil {
		return "", fmt.Errorf("render pending revision: %w", marshalError)
	}

	return string(payload), nil
}

// RecordCreation never fails the creation it follows: the thing is already stored, and reporting a failure would
// only make the assistant create it again. Forgetting it just means rewriting it later needs confirming.
func (assistantRevisionApplication *AssistantRevisionApplication) RecordCreation(
	executionContext context.Context,
	origin vo.AssistantQueryOriginVo,
	subjectKind vo.AssistantRevisionSubjectKindVo,
	subjectID uint,
) {
	if recordError := assistantRevisionApplication.assistantRevisionService.RecordCreatedSubject(
		executionContext, origin, subjectKind, subjectID); recordError != nil {
		log.Printf("remember %s %d created in conversation %d: %v",
			subjectKind, subjectID, origin.ConversationID, recordError)
	}
}

// ConfirmPendingRevision claims the proposal before writing, so it is written at most once, and hands it back to
// pending when the ordinary rewrite refuses it, so the owner can deal with the reason and confirm again.
func (assistantRevisionApplication *AssistantRevisionApplication) ConfirmPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	pendingRevision, findError := assistantRevisionApplication.assistantRevisionService.FindPendingRevision(
		executionContext, viewerID, id)
	if findError != nil {
		return dto.AssistantPendingRevisionDto{}, findError
	}

	applier, applierError := assistantRevisionApplication.applierFor(pendingRevision.SubjectKind)
	if applierError != nil {
		return dto.AssistantPendingRevisionDto{}, applierError
	}

	content := string(pendingRevision.Content)

	target, inspectError := applier.Inspect(executionContext, viewerID, content)
	if inspectError != nil {
		return dto.AssistantPendingRevisionDto{}, inspectError
	}

	confirmedRevision, claimError := assistantRevisionApplication.assistantRevisionService.ClaimConfirmation(
		executionContext, viewerID, id, target.UpdatedAt)
	if claimError != nil {
		return dto.AssistantPendingRevisionDto{}, claimError
	}

	if _, applyError := applier.Apply(executionContext, viewerID, content); applyError != nil {
		reopenError := assistantRevisionApplication.assistantRevisionService.ReopenPendingRevision(
			executionContext, id)

		return dto.AssistantPendingRevisionDto{}, errors.Join(applyError, reopenError)
	}

	return confirmedRevision, nil
}

func (assistantRevisionApplication *AssistantRevisionApplication) RejectPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	return assistantRevisionApplication.assistantRevisionService.RejectPendingRevision(
		executionContext, viewerID, id)
}

func (assistantRevisionApplication *AssistantRevisionApplication) applierFor(
	subjectKind string,
) (domaininterface.IAssistantRevisionApplier, error) {
	for _, applier := range assistantRevisionApplication.appliers {
		if string(applier.SubjectKind()) == subjectKind {
			return applier, nil
		}
	}

	return nil, fmt.Errorf("no assistant revision applier for %q", subjectKind)
}

package application

import (
	"context"
	"log"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type AssistantRevisionApplication struct {
	assistantRevisionService *service.AssistantRevisionService
}

func NewAssistantRevisionApplication(
	assistantRevisionService *service.AssistantRevisionService,
) *AssistantRevisionApplication {
	return &AssistantRevisionApplication{assistantRevisionService: assistantRevisionService}
}

func (assistantRevisionApplication *AssistantRevisionApplication) Revise(
	executionContext context.Context,
	origin vo.AssistantQueryOriginVo,
	subjectKind vo.AssistantRevisionSubjectKindVo,
	content string,
) (string, error) {
	return assistantRevisionApplication.assistantRevisionService.Revise(
		executionContext, origin, subjectKind, content)
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

func (assistantRevisionApplication *AssistantRevisionApplication) ConfirmPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	return assistantRevisionApplication.assistantRevisionService.ConfirmPendingRevision(
		executionContext, viewerID, id)
}

func (assistantRevisionApplication *AssistantRevisionApplication) RejectPendingRevision(
	executionContext context.Context, viewerID uint, id uint,
) (dto.AssistantPendingRevisionDto, error) {
	return assistantRevisionApplication.assistantRevisionService.RejectPendingRevision(
		executionContext, viewerID, id)
}

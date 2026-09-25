package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type AssistantConversationApplication struct {
	assistantConversationService *service.AssistantConversationService
}

func NewAssistantConversationApplication(
	assistantConversationService *service.AssistantConversationService,
) *AssistantConversationApplication {
	return &AssistantConversationApplication{assistantConversationService: assistantConversationService}
}

// Ask returns where the answer will appear; the answer itself is written asynchronously.
func (assistantConversationApplication *AssistantConversationApplication) Ask(
	executionContext context.Context, askDto dto.AssistantAskDto,
) (dto.AssistantAnswerStartedDto, error) {
	return assistantConversationApplication.assistantConversationService.Ask(executionContext, askDto)
}

// FailInterruptedAnswers fails answers the last shutdown cut off; it runs at startup because a crash never gets to tidy up.
func (assistantConversationApplication *AssistantConversationApplication) FailInterruptedAnswers(
	executionContext context.Context,
) (int, error) {
	return assistantConversationApplication.assistantConversationService.FailInterruptedAnswers(
		executionContext)
}

func (assistantConversationApplication *AssistantConversationApplication) ListConversations(
	executionContext context.Context, viewerID uint,
) ([]dto.ConversationSummaryDto, error) {
	return assistantConversationApplication.assistantConversationService.ListConversations(
		executionContext, viewerID)
}

func (assistantConversationApplication *AssistantConversationApplication) GetConversation(
	executionContext context.Context, viewerID uint, id uint,
) (dto.ConversationDto, error) {
	return assistantConversationApplication.assistantConversationService.GetConversation(
		executionContext, viewerID, id)
}

package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// AssistantConversationApplication orchestrates the conversation use cases. Each
// method is one call into the domain; no rule, ceiling or ordering decision lives
// here.
type AssistantConversationApplication struct {
	assistantConversationService *service.AssistantConversationService
}

func NewAssistantConversationApplication(
	assistantConversationService *service.AssistantConversationService,
) *AssistantConversationApplication {
	return &AssistantConversationApplication{assistantConversationService: assistantConversationService}
}

func (assistantConversationApplication *AssistantConversationApplication) Ask(
	executionContext context.Context, askDto dto.AssistantAskDto,
) (dto.AssistantAnswerDto, error) {
	return assistantConversationApplication.assistantConversationService.Ask(executionContext, askDto)
}

func (assistantConversationApplication *AssistantConversationApplication) ListConversations(
	executionContext context.Context,
) ([]dto.ConversationSummaryDto, error) {
	return assistantConversationApplication.assistantConversationService.ListConversations(executionContext)
}

func (assistantConversationApplication *AssistantConversationApplication) GetConversation(
	executionContext context.Context, id uint,
) (dto.ConversationDto, error) {
	return assistantConversationApplication.assistantConversationService.GetConversation(executionContext, id)
}

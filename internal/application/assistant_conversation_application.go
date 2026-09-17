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

// Ask hands the question over and gets back where its answer will appear. The answer
// itself is written afterwards, off this call.
func (assistantConversationApplication *AssistantConversationApplication) Ask(
	executionContext context.Context, askDto dto.AssistantAskDto,
) (dto.AssistantAnswerStartedDto, error) {
	return assistantConversationApplication.assistantConversationService.Ask(executionContext, askDto)
}

// FailInterruptedAnswers clears out the answers the last shutdown cut off, and says
// how many there were so that whoever starts the system can say it out loud.
//
// It is called on the way up rather than on the way down, because a shutdown is not
// always given the chance to tidy up — a power cut and a crash leave the same rows
// behind as a clean stop, and only the next start is guaranteed to happen.
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

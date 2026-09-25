package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ConversationDomain decides what the assistant remembers (only recent messages, to bound cost) versus what a person can read (everything), unfolding each stored exchange row into messages.
type ConversationDomain struct {
	conversation entities.Conversation
}

func NewConversationDomain(conversation entities.Conversation) ConversationDomain {
	return ConversationDomain{conversation: conversation}
}

// RequireOwnership returns the same not-found error for non-owners so identifiers cannot be probed; a zero viewer owns nothing.
func (conversationDomain ConversationDomain) RequireOwnership(viewerID uint) error {
	if viewerID == 0 || conversationDomain.conversation.OwnerID != viewerID {
		return ConversationNotFound(conversationDomain.conversation.ID)
	}

	return nil
}

// HasAnswerInFlight reports whether an answer is still being written, which refuses a new question; failed exchanges do not block.
func (conversationDomain ConversationDomain) HasAnswerInFlight() bool {
	for _, turn := range conversationDomain.conversation.Turns {
		if vo.NewAssistantTurnStatusVo(turn.Status) == vo.AssistantTurnRunning {
			return true
		}
	}

	return false
}

// RecentMessages returns the last limit answered messages, earliest first, excluding tool lookups and unanswered exchanges so the assistant does not think it declined them.
func (conversationDomain ConversationDomain) RecentMessages(limit int) []vo.AssistantMessageVo {
	answeredMessages := make([]conversationMessage, 0, len(conversationDomain.conversation.Turns)*2)
	for _, message := range conversationDomain.messages() {
		if message.Status != vo.AssistantTurnAnswered {
			continue
		}

		answeredMessages = append(answeredMessages, message)
	}

	firstIncluded := 0
	if len(answeredMessages) > limit {
		firstIncluded = len(answeredMessages) - limit
	}

	recentMessages := make([]vo.AssistantMessageVo, 0, len(answeredMessages)-firstIncluded)
	for _, message := range answeredMessages[firstIncluded:] {
		recentMessages = append(recentMessages, vo.AssistantMessageVo{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	return recentMessages
}

// ToDto includes running and failed exchanges with their status so a returning reader can tell "still going" from "broke" instead of re-asking.
func (conversationDomain ConversationDomain) ToDto() dto.ConversationDto {
	messages := conversationDomain.messages()

	messageDtos := make([]dto.ConversationMessageDto, 0, len(messages))
	for _, message := range messages {
		messageDtos = append(messageDtos, dto.ConversationMessageDto{
			Role:                string(message.Role),
			Content:             message.Content,
			CreatedAt:           message.CreatedAt.UTC(),
			Status:              string(message.Status),
			FailureReason:       message.FailureReason,
			QueryCount:          message.QueryCount,
			StoppedAtQueryLimit: message.StoppedAtQueryLimit,
			Usage:               message.Usage,
		})
	}

	return dto.ConversationDto{
		ID:           conversationDomain.conversation.ID,
		LastActiveAt: conversationDomain.conversation.LastActiveAt.UTC(),
		Messages:     messageDtos,
	}
}

// ToSummaryDto includes the message count, which distinguishes unnamed conversations at a glance.
func (conversationDomain ConversationDomain) ToSummaryDto() dto.ConversationSummaryDto {
	return dto.ConversationSummaryDto{
		ID:           conversationDomain.conversation.ID,
		LastActiveAt: conversationDomain.conversation.LastActiveAt.UTC(),
		MessageCount: len(conversationDomain.messages()),
	}
}

// conversationMessage is one unfolded message, shared by every view so the assistant's memory and the readable record never diverge.
type conversationMessage struct {
	Role          vo.AssistantMessageRole
	Content       string
	CreatedAt     time.Time
	Status        vo.AssistantTurnStatusVo
	FailureReason string
	// QueryCount, StoppedAtQueryLimit and Usage are filled only on answers.
	QueryCount          int
	StoppedAtQueryLimit bool
	Usage               int
}

// messages unfolds exchanges earliest first; an unanswered exchange yields only its question, carrying the status instead of a blank reply.
func (conversationDomain ConversationDomain) messages() []conversationMessage {
	messages := make([]conversationMessage, 0, len(conversationDomain.conversation.Turns)*2)
	for _, turn := range conversationDomain.conversation.Turns {
		status := vo.NewAssistantTurnStatusVo(turn.Status)

		messages = append(messages, conversationMessage{
			Role:          vo.AssistantMessageRoleAsk,
			Content:       turn.Ask,
			CreatedAt:     turn.CreatedAt,
			Status:        status,
			FailureReason: turn.FailureReason,
		})

		if status != vo.AssistantTurnAnswered {
			continue
		}

		messages = append(messages, conversationMessage{
			Role:                vo.AssistantMessageRoleAnswer,
			Content:             turn.Answer,
			CreatedAt:           turn.CreatedAt,
			Status:              status,
			QueryCount:          turn.QueryCount,
			StoppedAtQueryLimit: turn.StoppedAtQueryLimit,
			Usage:               turn.Usage,
		})
	}

	return messages
}

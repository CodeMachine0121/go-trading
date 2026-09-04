package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ConversationDomain holds one conversation and answers the two questions asked of
// it: what the assistant is allowed to remember, and what a person can still read.
//
// They are different answers on purpose. Only the most recent handful of messages is
// sent to the assistant, which is what keeps the cost of an exchange from growing
// with the length of the conversation; everything ever said stays readable. One
// number for both would have made a long conversation either ruinous or amnesiac in
// the record as well.
//
// An exchange is stored as one row holding a question and an answer, so the flat run
// of messages a reader expects is derived here rather than stored twice.
type ConversationDomain struct {
	conversation entities.Conversation
}

func NewConversationDomain(conversation entities.Conversation) ConversationDomain {
	return ConversationDomain{conversation: conversation}
}

// RecentMessages is what the assistant is shown: the last so many messages of this
// conversation, earliest first.
//
// Only questions and answers appear. What earlier exchanges looked up is left out
// deliberately — it was worth its cost once, when the answer that needed it was being
// written, and paying for it again on every later exchange buys nothing.
func (conversationDomain ConversationDomain) RecentMessages(limit int) []vo.AssistantMessageVo {
	messages := conversationDomain.messages()

	firstIncluded := 0
	if len(messages) > limit {
		firstIncluded = len(messages) - limit
	}

	recentMessages := make([]vo.AssistantMessageVo, 0, len(messages)-firstIncluded)
	for _, message := range messages[firstIncluded:] {
		recentMessages = append(recentMessages, vo.AssistantMessageVo{
			Role:    message.Role,
			Content: message.Content,
		})
	}

	return recentMessages
}

// ToDto is the whole conversation as it is handed outwards, every message included.
func (conversationDomain ConversationDomain) ToDto() dto.ConversationDto {
	messages := conversationDomain.messages()

	messageDtos := make([]dto.ConversationMessageDto, 0, len(messages))
	for _, message := range messages {
		messageDtos = append(messageDtos, dto.ConversationMessageDto{
			Role:      string(message.Role),
			Content:   message.Content,
			CreatedAt: message.CreatedAt.UTC(),
		})
	}

	return dto.ConversationDto{
		ID:           conversationDomain.conversation.ID,
		LastActiveAt: conversationDomain.conversation.LastActiveAt.UTC(),
		Messages:     messageDtos,
	}
}

// ToSummaryDto is this conversation as it appears in the list of them. The message
// count is what tells two conversations apart at a glance when neither has a name.
func (conversationDomain ConversationDomain) ToSummaryDto() dto.ConversationSummaryDto {
	return dto.ConversationSummaryDto{
		ID:           conversationDomain.conversation.ID,
		LastActiveAt: conversationDomain.conversation.LastActiveAt.UTC(),
		MessageCount: len(conversationDomain.messages()),
	}
}

// conversationMessage is one message once an exchange has been unfolded into the two
// it holds. It exists so that "the last twenty messages" and "every message" are cut
// from the same run rather than each unfolding the exchanges their own way — the day
// the two ways of unfolding disagree, the assistant remembers a conversation the
// record does not show.
type conversationMessage struct {
	Role      vo.AssistantMessageRole
	Content   string
	CreatedAt time.Time
}

// messages unfolds every exchange into the question and the answer it holds,
// earliest first. All three of the public answers above are cut from this one run.
func (conversationDomain ConversationDomain) messages() []conversationMessage {
	messages := make([]conversationMessage, 0, len(conversationDomain.conversation.Turns)*2)
	for _, turn := range conversationDomain.conversation.Turns {
		messages = append(messages,
			conversationMessage{
				Role:      vo.AssistantMessageRoleAsk,
				Content:   turn.Ask,
				CreatedAt: turn.CreatedAt,
			},
			conversationMessage{
				Role:      vo.AssistantMessageRoleAnswer,
				Content:   turn.Answer,
				CreatedAt: turn.CreatedAt,
			})
	}

	return messages
}

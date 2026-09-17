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

// RequireOwnership refuses anybody but the person who asked, with the same refusal
// a conversation that is not there gives.
//
// One sentence for both, exactly as strategy scripts do it: told apart, they would let
// somebody holding a list of identifiers learn which conversations exist and whose
// they are not.
//
// A viewer of nobody owns nothing. Saying so here rather than trusting the caller
// matters because a conversation whose owner column somehow held nothing would
// otherwise belong to every unidentified request at once.
func (conversationDomain ConversationDomain) RequireOwnership(viewerID uint) error {
	if viewerID == 0 || conversationDomain.conversation.OwnerID != viewerID {
		return ConversationNotFound(conversationDomain.conversation.ID)
	}

	return nil
}

// HasAnswerInFlight says whether an answer is being written on this conversation
// right now.
//
// It is asked before a new question is accepted, and a yes refuses it. Two answers
// being written into one conversation at once leaves nobody able to say which of
// them the record belongs to — and the moment somebody would do it is almost always
// the one this whole design exists to remove: believing the first one never sent.
//
// A failed exchange is not in flight and does not block anything. Nothing is still
// being written, so there is nothing a second question could collide with.
func (conversationDomain ConversationDomain) HasAnswerInFlight() bool {
	for _, turn := range conversationDomain.conversation.Turns {
		if vo.NewAssistantTurnStatusVo(turn.Status) == vo.AssistantTurnRunning {
			return true
		}
	}

	return false
}

// NewestTurnID names the exchange most recently added to this conversation.
//
// It is how the exchange just started is found again, so that the answer can be
// written back over the very row that reserved the place. Turns arrive earliest
// first, so the newest is the last of them; a conversation with none answers zero,
// which no exchange carries.
func (conversationDomain ConversationDomain) NewestTurnID() uint {
	if len(conversationDomain.conversation.Turns) == 0 {
		return 0
	}

	return conversationDomain.conversation.Turns[len(conversationDomain.conversation.Turns)-1].ID
}

// RecentMessages is what the assistant is shown: the last so many messages of this
// conversation, earliest first.
//
// Only questions and answers appear. What earlier exchanges looked up is left out
// deliberately — it was worth its cost once, when the answer that needed it was being
// written, and paying for it again on every later exchange buys nothing.
//
// Exchanges that never produced an answer are left out too, questions included. A
// question with nothing under it reads to the assistant as one it declined to answer,
// and it will go on to explain why it declined — which is not what happened.
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

// ToDto is the whole conversation as it is handed outwards, every message included —
// the exchanges still being written and the ones that failed among them.
//
// Those two are why each message says what state its exchange is in. A reader coming
// back to a conversation has to be able to tell "still going" from "it broke" from
// "here is your answer", and the alternative — leaving the unfinished ones out — is
// exactly the blank screen that makes somebody ask the same question twice.
func (conversationDomain ConversationDomain) ToDto() dto.ConversationDto {
	messages := conversationDomain.messages()

	messageDtos := make([]dto.ConversationMessageDto, 0, len(messages))
	for _, message := range messages {
		messageDtos = append(messageDtos, dto.ConversationMessageDto{
			Role:          string(message.Role),
			Content:       message.Content,
			CreatedAt:     message.CreatedAt.UTC(),
			Status:        string(message.Status),
			FailureReason: message.FailureReason,
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
	// Status is where the exchange this message belongs to has got to, and
	// FailureReason the one sentence explaining a failed one.
	Status        vo.AssistantTurnStatusVo
	FailureReason string
}

// messages unfolds every exchange into the messages it holds, earliest first. All
// three of the public answers above are cut from this one run.
//
// **An exchange without an answer yields only its question.** Emitting an empty
// answer alongside it would put a blank reply into the record, and every reader —
// the screen included — would have to know to suppress it. One message carrying the
// state says the same thing with nothing to suppress: the question is there, its
// exchange is running or failed, and where the reply would go there is a wait or a
// reason instead.
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
			Role:      vo.AssistantMessageRoleAnswer,
			Content:   turn.Answer,
			CreatedAt: turn.CreatedAt,
			Status:    status,
		})
	}

	return messages
}

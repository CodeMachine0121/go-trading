package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_conversation_repository.go -destination=mocks/mock_i_conversation_repository.go -package=mocks

// IConversationRepository stores and retrieves conversations.
//
// One exchange is one write. A question and the answer to it live and die together —
// an assistant that never answers must leave nothing behind — and that guarantee is
// only free while storing them is a single statement.
//
// When a conversation was last active is the moment of the exchange that moved it.
// Adding an exchange therefore does not take that moment as a separate argument:
// they are the same fact, and a caller allowed to disagree with itself about it
// eventually will.
//
// Summing usage lives here rather than on a repository of its own, because there is
// no such thing as a stored day: usage belongs to an exchange, and an exchange
// belongs to a conversation.
type IConversationRepository interface {
	// Save stores a new conversation together with its first exchange, and hands it
	// back as stored, identifier and times filled in.
	Save(executionContext context.Context, conversation entities.Conversation) (entities.Conversation, error)
	// AppendTurn adds one exchange to the conversation this identifier names and
	// hands the conversation back as it now stands. Refuses with
	// ErrConversationNotFound when there is no such conversation.
	AppendTurn(
		executionContext context.Context, conversationId uint, turn entities.AssistantTurn,
	) (entities.Conversation, error)
	// FindOne returns the conversation carrying this identifier with every exchange
	// under it, earliest first, or ErrConversationNotFound.
	FindOne(executionContext context.Context, id uint) (entities.Conversation, error)
	// FindAll returns every conversation, the most recently active first.
	FindAll(executionContext context.Context) ([]entities.Conversation, error)
	// SumUsageBetween totals the usage of every exchange stored in this stretch,
	// start included and end excluded. Holding none is a total of zero rather than a
	// failure.
	SumUsageBetween(executionContext context.Context, from time.Time, to time.Time) (int, error)
}

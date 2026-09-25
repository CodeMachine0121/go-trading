package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_conversation_repository.go -destination=mocks/mock_i_conversation_repository.go -package=mocks

// IConversationRepository writes an exchange twice (on acceptance and on completion) so in-progress answers are visible and restart-sweepable.
type IConversationRepository interface {
	Save(executionContext context.Context, conversation entities.Conversation) (entities.Conversation, error)
	// AppendTurn returns the stored exchange itself (not the conversation) so concurrent questions each get their own row; ErrConversationNotFound if absent.
	AppendTurn(
		executionContext context.Context, conversationId uint, turn entities.AssistantTurn,
	) (entities.AssistantTurn, error)
	// FindOne returns exchanges earliest first, or ErrConversationNotFound.
	FindOne(executionContext context.Context, id uint) (entities.Conversation, error)
	// FindAllOwnedBy filters by owner in the query itself, most recently active first.
	FindAllOwnedBy(executionContext context.Context, ownerID uint) ([]entities.Conversation, error)
	// CompleteTurn writes only the ending fields (answer, cost, lookups, failure), never the question; ErrConversationNotFound means the conversation was deleted mid-answer.
	CompleteTurn(executionContext context.Context, turn entities.AssistantTurn) error
	// FailAllRunningTurns runs at startup because in-flight answers live only in memory and die with the process.
	FailAllRunningTurns(executionContext context.Context, reason string) (int, error)
	// SumUsageBetween sums [from, to); failed exchanges record no usage.
	SumUsageBetween(executionContext context.Context, from time.Time, to time.Time) (int, error)
}

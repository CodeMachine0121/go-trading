package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_conversation_repository.go -destination=mocks/mock_i_conversation_repository.go -package=mocks

// IConversationRepository stores and retrieves conversations.
//
// An exchange is written twice: once when the question is accepted, and once when
// the answer is finished or has failed. That is not a lost guarantee but the point of
// the design — the first write is what makes an answer visible while it is still
// being written, and what a restart can sweep up after.
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
	// FindAllOwnedBy returns this person's conversations, the most recently active
	// first. Whose they are is a condition on the read rather than a filter applied
	// after it: a list that arrives whole and is narrowed afterwards is a list that
	// was somewhere in memory in full, and one forgotten narrowing away from being
	// handed over.
	FindAllOwnedBy(executionContext context.Context, ownerID uint) ([]entities.Conversation, error)
	// CompleteTurn writes an answer, or a failure, over the exchange the turn's
	// identifier names.
	//
	// Only what an ending settles is written: the answer, where it got to, what it
	// cost, how many lookups it ran and why it failed. The question and the moment it
	// was asked are left alone — they were settled when the exchange began, and
	// rewriting them would be a second chance to get them wrong.
	//
	// Refuses with ErrConversationNotFound when no exchange carries that identifier,
	// which is what a conversation deleted mid-answer looks like from here.
	CompleteTurn(executionContext context.Context, turn entities.AssistantTurn) error
	// FailAllRunningTurns marks every exchange still recorded as running as failed,
	// carrying this reason, and says how many it touched.
	//
	// It runs at startup. An answer being written lives in this system's memory and
	// nowhere else, so every one of them died with the last shutdown — and a row left
	// at running is a wait nobody can end and a conversation nobody can add to.
	FailAllRunningTurns(executionContext context.Context, reason string) (int, error)
	// SumUsageBetween totals the usage of every exchange stored in this stretch,
	// start included and end excluded. Holding none is a total of zero rather than a
	// failure.
	//
	// Exchanges that failed contribute nothing because nothing was recorded against
	// them: nobody is charged for an answer they never got.
	SumUsageBetween(executionContext context.Context, from time.Time, to time.Time) (int, error)
}

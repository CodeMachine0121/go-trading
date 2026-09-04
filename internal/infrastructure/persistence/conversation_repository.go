package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// conversationTurnsAssociation is how GORM is asked for a conversation's exchanges,
// and conversationTurnQueriesAssociation for what each of those exchanges looked at.
// They are written once here so that every read fetches them the same way — a
// conversation read back without its exchanges looks like a conversation nobody ever
// used.
const (
	conversationTurnsAssociation       = "Turns"
	conversationTurnQueriesAssociation = "Turns.Queries"
)

// ConversationRepository stores conversations in PostgreSQL.
type ConversationRepository struct {
	database *gorm.DB
}

func NewConversationRepository(database *gorm.DB) *ConversationRepository {
	return &ConversationRepository{database: database}
}

// Save stores a new conversation together with its first exchange.
func (conversationRepository *ConversationRepository) Save(
	executionContext context.Context, conversation entities.Conversation,
) (entities.Conversation, error) {
	result := conversationRepository.database.WithContext(executionContext).Create(&conversation)
	if result.Error != nil {
		return entities.Conversation{}, fmt.Errorf("save conversation: %w", result.Error)
	}

	return conversation, nil
}

// AppendTurn adds one exchange to a conversation and hands the conversation back as
// it now stands.
//
// Moving the conversation's last-active moment is done first, and it is also how a
// conversation that is not there is reported: no row moved means no such
// conversation. Asking whether it exists and then writing would let a deletion land
// between the two and leave an exchange belonging to nothing.
//
// The write and the read-back share one transaction, so what comes back is what this
// call stored rather than whatever a second question arriving at the same moment left
// behind.
func (conversationRepository *ConversationRepository) AppendTurn(
	executionContext context.Context, conversationId uint, turn entities.AssistantTurn,
) (entities.Conversation, error) {
	appendedConversation := entities.Conversation{}

	transactionError := conversationRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			moved := transaction.
				Model(&entities.Conversation{ID: conversationId}).
				Update("last_active_at", turn.CreatedAt)
			if moved.Error != nil {
				return moved.Error
			}
			if moved.RowsAffected == 0 {
				return domains.ConversationNotFound(conversationId)
			}

			turn.ConversationID = conversationId
			if created := transaction.Create(&turn); created.Error != nil {
				return created.Error
			}

			readBackConversation, readBackError := readConversation(transaction, conversationId)
			if readBackError != nil {
				return readBackError
			}

			appendedConversation = readBackConversation

			return nil
		})

	// A conversation that is not there is the one refusal this method owes the caller
	// in its own words; everything else is a storage failure and is said so. The
	// wrapping happens out here rather than at each statement because the transaction
	// can also fail before any of them runs — and that failure, left bare, would
	// reach the caller as a sentence from the database driver.
	if transactionError != nil {
		if errors.Is(transactionError, domains.ErrConversationNotFound) {
			return entities.Conversation{}, transactionError
		}

		return entities.Conversation{}, fmt.Errorf("append conversation turn: %w", transactionError)
	}

	return appendedConversation, nil
}

// FindOne returns the conversation carrying this identifier with every exchange under
// it, earliest first.
func (conversationRepository *ConversationRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.Conversation, error) {
	return readConversation(conversationRepository.database.WithContext(executionContext), id)
}

// FindAll returns every conversation, the most recently active first.
//
// The exchanges come along because the list says how many messages each conversation
// holds, and that number is what tells two of them apart when neither has a name.
// What each exchange looked at does not: nobody reads a lookup from a list.
func (conversationRepository *ConversationRepository) FindAll(
	executionContext context.Context,
) ([]entities.Conversation, error) {
	conversations := make([]entities.Conversation, 0)

	result := conversationRepository.database.WithContext(executionContext).
		Preload(conversationTurnsAssociation, orderedTurns).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "last_active_at"}, Desc: true}).
		Find(&conversations)
	if result.Error != nil {
		return nil, fmt.Errorf("find conversations: %w", result.Error)
	}

	return conversations, nil
}

// SumUsageBetween totals what the assistant was used for over this stretch, start
// included and end excluded.
//
// The exchanges' usage is read and added up here rather than summed by the database.
// A day of exchanges is bounded by the very allowance this total is compared against,
// so the row count cannot run away — and adding them up in code keeps data access on
// the typed API instead of a hand-written aggregate.
func (conversationRepository *ConversationRepository) SumUsageBetween(
	executionContext context.Context, from time.Time, to time.Time,
) (int, error) {
	usages := make([]int, 0)

	result := conversationRepository.database.WithContext(executionContext).
		Model(&entities.AssistantTurn{}).
		Where(clause.Gte{Column: "created_at", Value: from}).
		Where(clause.Lt{Column: "created_at", Value: to}).
		Pluck("usage", &usages)
	if result.Error != nil {
		return 0, fmt.Errorf("sum assistant usage: %w", result.Error)
	}

	total := 0
	for _, usage := range usages {
		total += usage
	}

	return total, nil
}

// readConversation is one conversation with everything under it, in the order it
// happened. Both the read-back after a write and the plain read use it, so that a
// conversation never comes back looking different depending on which asked.
func readConversation(database *gorm.DB, id uint) (entities.Conversation, error) {
	conversation := entities.Conversation{}

	result := database.
		Preload(conversationTurnsAssociation, orderedTurns).
		Preload(conversationTurnQueriesAssociation, orderedQueryRecords).
		First(&conversation, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.Conversation{}, domains.ConversationNotFound(id)
	}
	if result.Error != nil {
		return entities.Conversation{}, fmt.Errorf("find conversation: %w", result.Error)
	}

	return conversation, nil
}

// orderedTurns reads a conversation's exchanges earliest first. Read shuffled, a
// conversation stops being a conversation.
func orderedTurns(database *gorm.DB) *gorm.DB {
	return database.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}})
}

// orderedQueryRecords reads an exchange's lookups in the order they were made,
// because a chain of reasoning read out of order looks like a set of unrelated
// lookups.
func orderedQueryRecords(database *gorm.DB) *gorm.DB {
	return database.Order(clause.OrderByColumn{Column: clause.Column{Name: "sequence"}})
}

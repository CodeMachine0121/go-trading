package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/jackc/pgx/v5/pgconn"
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

// AppendTurn adds one exchange to a conversation and hands that exchange back as
// stored.
//
// Moving the conversation's last-active moment is done first, and it is also how a
// conversation that is not there is reported: no row moved means no such
// conversation. Asking whether it exists and then writing would let a deletion land
// between the two and leave an exchange belonging to nothing.
//
// What comes back is the row this call created, named by the identifier the store
// gave it. It is not found by reading the conversation back and taking the last
// exchange: two questions arriving at once would both read whichever row committed
// second, and one answer would be written over the other.
func (conversationRepository *ConversationRepository) AppendTurn(
	executionContext context.Context, conversationId uint, turn entities.AssistantTurn,
) (entities.AssistantTurn, error) {
	appendedTurn := entities.AssistantTurn{}

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
				return conversationRepository.appendFailureOf(created.Error)
			}

			appendedTurn = turn

			return nil
		})

	// A conversation that is not there is the one refusal this method owes the caller
	// in its own words; everything else is a storage failure and is said so. The
	// wrapping happens out here rather than at each statement because the transaction
	// can also fail before any of them runs — and that failure, left bare, would
	// reach the caller as a sentence from the database driver.
	if transactionError != nil {
		if errors.Is(transactionError, domains.ErrConversationNotFound) ||
			errors.Is(transactionError, domains.ErrAssistantAnswerInProgress) {
			return entities.AssistantTurn{}, transactionError
		}

		return entities.AssistantTurn{}, fmt.Errorf("append conversation turn: %w", transactionError)
	}

	return appendedTurn, nil
}

// appendFailureOf turns a failed insert into the refusal it actually is.
//
// One broken index means a second answer was starting on a conversation that already
// had one — two requests that both read "nothing in flight" before either had
// written. That is a person asking twice, and they get the same sentence as the
// person who was merely a moment slower. Anything else is a fault, and dressing it up
// as "wait for the previous one" would leave somebody waiting for an answer that is
// never coming.
func (conversationRepository *ConversationRepository) appendFailureOf(writeError error) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == AssistantTurnOneRunningPerConversationIndex {
		return domains.AssistantAnswerInProgress()
	}

	return writeError
}

// CompleteTurn writes an answer, or a failure, over the exchange this turn names.
//
// The columns are listed rather than the struct handed over, because an ending
// settles only some of them: GORM writing the whole struct would blank the question
// and reset the moment it was asked, both of which were settled when the exchange
// began.
//
// The lookups it made are created alongside, in the same transaction as the update.
// A record of what an answer read that outlived the answer failing to save would
// describe reasoning behind an answer nobody has.
func (conversationRepository *ConversationRepository) CompleteTurn(
	executionContext context.Context, turn entities.AssistantTurn,
) error {
	transactionError := conversationRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			completed := transaction.
				Model(&entities.AssistantTurn{ID: turn.ID}).
				Select(
					"answer", "status", "failure_reason",
					"usage", "query_count", "stopped_at_query_limit",
				).
				Updates(entities.AssistantTurn{
					Answer:              turn.Answer,
					Status:              turn.Status,
					FailureReason:       turn.FailureReason,
					Usage:               turn.Usage,
					QueryCount:          turn.QueryCount,
					StoppedAtQueryLimit: turn.StoppedAtQueryLimit,
				})
			if completed.Error != nil {
				return completed.Error
			}
			// The lookup was on the exchange, not the conversation, so that is what
			// the refusal names. A completion carries no conversation identifier —
			// it was settled when the place was reserved — so reporting one here
			// would always say "conversation 0".
			if completed.RowsAffected == 0 {
				return domains.AssistantTurnNotFound(turn.ID)
			}

			if len(turn.Queries) == 0 {
				return nil
			}

			queries := make([]entities.AssistantQueryRecord, 0, len(turn.Queries))
			for _, query := range turn.Queries {
				query.AssistantTurnID = turn.ID
				queries = append(queries, query)
			}

			return transaction.Create(&queries).Error
		})

	// A conversation that is no longer there is the one refusal this method owes the
	// caller in its own words; everything else is a storage failure and is said so.
	if transactionError != nil {
		if errors.Is(transactionError, domains.ErrConversationNotFound) {
			return transactionError
		}

		return fmt.Errorf("complete conversation turn: %w", transactionError)
	}

	return nil
}

// FailAllRunningTurns marks every exchange still recorded as running as failed.
//
// It is one statement rather than a read followed by writes, because there is no
// decision to make per row: every one of them is stale by definition, since the
// process that was writing it no longer exists.
func (conversationRepository *ConversationRepository) FailAllRunningTurns(
	executionContext context.Context, reason string,
) (int, error) {
	swept := conversationRepository.database.WithContext(executionContext).
		Model(&entities.AssistantTurn{}).
		Where(clause.Eq{Column: "status", Value: string(vo.AssistantTurnRunning)}).
		Select("status", "failure_reason").
		Updates(entities.AssistantTurn{
			Status:        string(vo.AssistantTurnFailed),
			FailureReason: reason,
		})
	if swept.Error != nil {
		return 0, fmt.Errorf("fail running conversation turns: %w", swept.Error)
	}

	return int(swept.RowsAffected), nil
}

// FindOne returns the conversation carrying this identifier with every exchange under
// it, earliest first.
func (conversationRepository *ConversationRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.Conversation, error) {
	return readConversation(conversationRepository.database.WithContext(executionContext), id)
}

// FindAllOwnedBy returns this person's conversations, the most recently active first.
//
// The exchanges come along because the list says how many messages each conversation
// holds, and that number is what tells two of them apart when neither has a name.
// What each exchange looked at does not: nobody reads a lookup from a list.
func (conversationRepository *ConversationRepository) FindAllOwnedBy(
	executionContext context.Context, ownerID uint,
) ([]entities.Conversation, error) {
	conversations := make([]entities.Conversation, 0)

	result := conversationRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
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

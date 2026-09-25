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

// The association names are shared so every read preloads exchanges and their queries the same way.
const (
	conversationTurnsAssociation       = "Turns"
	conversationTurnQueriesAssociation = "Turns.Queries"
)

type ConversationRepository struct {
	database *gorm.DB
}

func NewConversationRepository(database *gorm.DB) *ConversationRepository {
	return &ConversationRepository{database: database}
}

func (conversationRepository *ConversationRepository) Save(
	executionContext context.Context, conversation entities.Conversation,
) (entities.Conversation, error) {
	result := conversationRepository.database.WithContext(executionContext).Create(&conversation)
	if result.Error != nil {
		return entities.Conversation{}, fmt.Errorf("save conversation: %w", result.Error)
	}

	return conversation, nil
}

// AppendTurn bumps the conversation's last-active time first, treating zero affected rows as not found to avoid a race with deletion, and returns the created row by its own ID rather than re-reading the last turn.
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

	// Wrapped here rather than per statement because the transaction itself can fail before any statement runs.
	if transactionError != nil {
		if errors.Is(transactionError, domains.ErrConversationNotFound) ||
			errors.Is(transactionError, domains.ErrAssistantAnswerInProgress) {
			return entities.AssistantTurn{}, transactionError
		}

		return entities.AssistantTurn{}, fmt.Errorf("append conversation turn: %w", transactionError)
	}

	return appendedTurn, nil
}

// appendFailureOf maps a unique-index violation to the "answer already in flight" refusal; any other error stays a storage failure.
func (conversationRepository *ConversationRepository) appendFailureOf(writeError error) error {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if isPostgresError &&
		postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == AssistantTurnOneRunningPerConversationIndex {
		return domains.AssistantAnswerInProgress()
	}

	return writeError
}

// CompleteTurn updates only the settling columns, so the question and its timestamp are not blanked, and creates the query records in the same transaction.
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
			// Name the turn, not the conversation, since a completion carries no conversation ID.
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

	if transactionError != nil {
		if errors.Is(transactionError, domains.ErrConversationNotFound) {
			return transactionError
		}

		return fmt.Errorf("complete conversation turn: %w", transactionError)
	}

	return nil
}

// FailAllRunningTurns fails every running turn in one statement, since all are stale after a restart.
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

func (conversationRepository *ConversationRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.Conversation, error) {
	return readConversation(conversationRepository.database.WithContext(executionContext), id)
}

// FindAllOwnedBy returns conversations most recently active first, preloading turns for message counts but not their queries.
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

// SumUsageBetween sums usage in [start, end) in code rather than SQL, which keeps to the typed API and is bounded by the daily allowance.
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

// readConversation is shared by every read so a conversation always comes back in the same shape.
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

func orderedTurns(database *gorm.DB) *gorm.DB {
	return database.Order(clause.OrderByColumn{Column: clause.Column{Name: "id"}})
}

func orderedQueryRecords(database *gorm.DB) *gorm.DB {
	return database.Order(clause.OrderByColumn{Column: clause.Column{Name: "sequence"}})
}

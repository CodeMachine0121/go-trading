package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PendingMessageRepository struct {
	database ambientTransactionDatabase
}

func NewPendingMessageRepository(database *gorm.DB) *PendingMessageRepository {
	return &PendingMessageRepository{database: ambientTransactionDatabase{root: database}}
}

// Enqueue leans on the round's unique index, so a round booked twice still queues one message.
func (pendingMessageRepository *PendingMessageRepository) Enqueue(
	executionContext context.Context, pendingMessage entities.PendingMessage,
) error {
	result := pendingMessageRepository.database.within(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "strategy_bot_id"}, {Name: "round_due_at"}},
			DoNothing: true,
		}).
		Create(&pendingMessage)
	if result.Error != nil {
		return fmt.Errorf("enqueue pending message: %w", result.Error)
	}

	return nil
}

// FindDispatchCandidates reads the head of each person's queue in its own step, so the second read is bounded by people, not by how much one of them has queued.
func (pendingMessageRepository *PendingMessageRepository) FindDispatchCandidates(
	executionContext context.Context, moment time.Time, limit int,
) ([]entities.PendingMessage, error) {
	unsettled := clause.Or(
		clause.Eq{Column: "status", Value: string(vo.PendingMessageReady)},
		clause.Eq{Column: "status", Value: string(vo.PendingMessageSending)},
	)

	headIDs := []uint{}
	if headError := pendingMessageRepository.database.within(executionContext).
		Model(&entities.PendingMessage{}).
		Where(unsettled).
		Group("recipient_user_id").
		Pluck("MIN(id)", &headIDs).Error; headError != nil {
		return nil, fmt.Errorf("find heads of pending message queues: %w", headError)
	}

	candidates := []entities.PendingMessage{}
	candidateCondition := clause.Expression(clause.Lte{Column: "expires_at", Value: moment.UTC()})
	if len(headIDs) > 0 {
		// GORM's IN clause takes its values loosely typed, as elsewhere in this package.
		headIDValues := make([]any, 0, len(headIDs))
		for _, headID := range headIDs {
			headIDValues = append(headIDValues, headID)
		}
		candidateCondition = clause.Or(clause.IN{Column: "id", Values: headIDValues}, candidateCondition)
	}

	result := pendingMessageRepository.database.within(executionContext).
		Where(unsettled).
		Where(candidateCondition).
		Order("id ASC").
		Limit(limit).
		Find(&candidates)
	if result.Error != nil {
		return nil, fmt.Errorf("find pending message candidates: %w", result.Error)
	}

	return candidates, nil
}

// Claim is one conditional update, so of two replicas reaching for one message exactly one gets it.
func (pendingMessageRepository *PendingMessageRepository) Claim(
	executionContext context.Context, id uint, claimant string, moment time.Time, claimedUntil time.Time,
) (bool, error) {
	claimedUntilUtc := claimedUntil.UTC()

	result := pendingMessageRepository.database.within(executionContext).
		Model(&entities.PendingMessage{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Or(
			clause.And(
				clause.Eq{Column: "status", Value: string(vo.PendingMessageReady)},
				clause.Lte{Column: "next_attempt_at", Value: moment.UTC()},
			),
			clause.And(
				clause.Eq{Column: "status", Value: string(vo.PendingMessageSending)},
				clause.Lte{Column: "claimed_until", Value: moment.UTC()},
			),
		)).
		Select("status", "claimed_by", "claimed_until").
		UpdateColumns(entities.PendingMessage{
			Status: string(vo.PendingMessageSending), ClaimedBy: claimant, ClaimedUntil: &claimedUntilUtc,
		})
	if result.Error != nil {
		return false, fmt.Errorf("claim pending message: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

func (pendingMessageRepository *PendingMessageRepository) MarkSent(
	executionContext context.Context, id uint, claimant string, settledAt time.Time,
) error {
	settledAtUtc := settledAt.UTC()

	return pendingMessageRepository.settle(executionContext, id, claimant, "mark pending message sent",
		[]string{"status", "claimed_by", "claimed_until", "settled_at"},
		entities.PendingMessage{Status: string(vo.PendingMessageSent), SettledAt: &settledAtUtc})
}

func (pendingMessageRepository *PendingMessageRepository) Reschedule(
	executionContext context.Context, id uint, claimant string, attemptCount int, nextAttemptAt time.Time,
) error {
	return pendingMessageRepository.settle(executionContext, id, claimant, "reschedule pending message",
		[]string{"status", "claimed_by", "claimed_until", "attempt_count", "next_attempt_at"},
		entities.PendingMessage{
			Status: string(vo.PendingMessageReady), AttemptCount: attemptCount, NextAttemptAt: nextAttemptAt.UTC(),
		})
}

func (pendingMessageRepository *PendingMessageRepository) Abandon(
	executionContext context.Context, id uint, claimant string, reason vo.PendingMessageAbandonReasonVo,
	settledAt time.Time,
) error {
	settledAtUtc := settledAt.UTC()

	return pendingMessageRepository.settle(executionContext, id, claimant, "abandon pending message",
		[]string{"status", "claimed_by", "claimed_until", "abandon_reason", "settled_at"},
		entities.PendingMessage{
			Status: string(vo.PendingMessageAbandoned), AbandonReason: string(reason), SettledAt: &settledAtUtc,
		})
}

// settle is shared by every write that ends a send, so all three are guarded by the sender still holding the message.
func (pendingMessageRepository *PendingMessageRepository) settle(
	executionContext context.Context, id uint, claimant string, action string,
	columns []string, settled entities.PendingMessage,
) error {
	result := pendingMessageRepository.database.within(executionContext).
		Model(&entities.PendingMessage{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "status", Value: string(vo.PendingMessageSending)}).
		Where(clause.Eq{Column: "claimed_by", Value: claimant}).
		Select(columns).
		UpdateColumns(settled)
	if result.Error != nil {
		return fmt.Errorf("%s: %w", action, result.Error)
	}

	return nil
}

func (pendingMessageRepository *PendingMessageRepository) DeleteSettledBefore(
	executionContext context.Context, cutoff time.Time,
) error {
	result := pendingMessageRepository.database.within(executionContext).
		Where(clause.Lt{Column: "settled_at", Value: cutoff.UTC()}).
		Delete(&entities.PendingMessage{})
	if result.Error != nil {
		return fmt.Errorf("delete settled pending messages: %w", result.Error)
	}

	return nil
}

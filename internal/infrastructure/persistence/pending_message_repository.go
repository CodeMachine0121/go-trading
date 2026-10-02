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

func (pendingMessageRepository *PendingMessageRepository) FindUnsettled(
	executionContext context.Context, limit int,
) ([]entities.PendingMessage, error) {
	pendingMessages := []entities.PendingMessage{}

	result := pendingMessageRepository.database.within(executionContext).
		Where(clause.Or(
			clause.Eq{Column: "status", Value: string(vo.PendingMessageReady)},
			clause.Eq{Column: "status", Value: string(vo.PendingMessageSending)},
		)).
		Order("id ASC").
		Limit(limit).
		Find(&pendingMessages)
	if result.Error != nil {
		return nil, fmt.Errorf("find unsettled pending messages: %w", result.Error)
	}

	return pendingMessages, nil
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
	executionContext context.Context, id uint, claimant string, reason string, settledAt time.Time,
) error {
	settledAtUtc := settledAt.UTC()

	return pendingMessageRepository.settle(executionContext, id, claimant, "abandon pending message",
		[]string{"status", "claimed_by", "claimed_until", "abandon_reason", "settled_at"},
		entities.PendingMessage{
			Status: string(vo.PendingMessageAbandoned), AbandonReason: reason, SettledAt: &settledAtUtc,
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

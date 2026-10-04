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

type ContractAutoOrderRepository struct {
	database ambientTransactionDatabase
}

func NewContractAutoOrderRepository(database *gorm.DB) *ContractAutoOrderRepository {
	return &ContractAutoOrderRepository{database: ambientTransactionDatabase{root: database}}
}

// Enqueue leans on the round's unique index, so a round booked twice still queues one order.
func (contractAutoOrderRepository *ContractAutoOrderRepository) Enqueue(
	executionContext context.Context, contractAutoOrder entities.ContractAutoOrder,
) error {
	result := contractAutoOrderRepository.database.within(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "strategy_bot_id"}, {Name: "round_due_at"}},
			DoNothing: true,
		}).
		Create(&contractAutoOrder)
	if result.Error != nil {
		return fmt.Errorf("enqueue contract auto order: %w", result.Error)
	}

	return nil
}

// FindDispatchCandidates reads the head of each bot's queue in its own step, so a bot whose order is still running holds back its later ones.
func (contractAutoOrderRepository *ContractAutoOrderRepository) FindDispatchCandidates(
	executionContext context.Context, limit int,
) ([]entities.ContractAutoOrder, error) {
	unsettled := clause.Or(
		clause.Eq{Column: "status", Value: string(vo.ContractAutoOrderReady)},
		clause.Eq{Column: "status", Value: string(vo.ContractAutoOrderExecuting)},
	)

	// MIN(id) per bot is an aggregate GORM has no typed form for; the fragment names columns only and takes no input, as in the pending message queue.
	headIDs := []uint{}
	if headError := contractAutoOrderRepository.database.within(executionContext).
		Model(&entities.ContractAutoOrder{}).
		Where(unsettled).
		Group("strategy_bot_id").
		Pluck("MIN(id)", &headIDs).Error; headError != nil {
		return nil, fmt.Errorf("find heads of contract auto order queues: %w", headError)
	}

	candidates := []entities.ContractAutoOrder{}
	if len(headIDs) == 0 {
		return candidates, nil
	}

	// GORM's IN clause takes its values loosely typed, as elsewhere in this package.
	headIDValues := make([]any, 0, len(headIDs))
	for _, headID := range headIDs {
		headIDValues = append(headIDValues, headID)
	}

	result := contractAutoOrderRepository.database.within(executionContext).
		Where(clause.IN{Column: "id", Values: headIDValues}).
		Order("id ASC").
		Limit(limit).
		Find(&candidates)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract auto order candidates: %w", result.Error)
	}

	return candidates, nil
}

// Claim is one conditional update, so of two replicas reaching for one order exactly one gets it.
func (contractAutoOrderRepository *ContractAutoOrderRepository) Claim(
	executionContext context.Context, id uint, claimant string, moment time.Time, claimedUntil time.Time,
) (bool, error) {
	claimedUntilUtc := claimedUntil.UTC()

	result := contractAutoOrderRepository.database.within(executionContext).
		Model(&entities.ContractAutoOrder{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Or(
			clause.And(
				clause.Eq{Column: "status", Value: string(vo.ContractAutoOrderReady)},
				clause.Lte{Column: "next_attempt_at", Value: moment.UTC()},
			),
			clause.And(
				clause.Eq{Column: "status", Value: string(vo.ContractAutoOrderExecuting)},
				clause.Lte{Column: "claimed_until", Value: moment.UTC()},
			),
		)).
		Select("status", "claimed_by", "claimed_until").
		UpdateColumns(entities.ContractAutoOrder{
			Status: string(vo.ContractAutoOrderExecuting), ClaimedBy: claimant, ClaimedUntil: &claimedUntilUtc,
		})
	if result.Error != nil {
		return false, fmt.Errorf("claim contract auto order: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

// SaveProgress names every column that moves after queuing, so empty values are written rather than skipped; only the holder of the claim it was taken under can write.
func (contractAutoOrderRepository *ContractAutoOrderRepository) SaveProgress(
	executionContext context.Context, contractAutoOrder entities.ContractAutoOrder, claimant string,
) (bool, error) {
	result := contractAutoOrderRepository.database.within(executionContext).
		Model(&entities.ContractAutoOrder{}).
		Where(clause.Eq{Column: "id", Value: contractAutoOrder.ID}).
		Where(clause.Eq{Column: "status", Value: string(vo.ContractAutoOrderExecuting)}).
		Where(clause.Eq{Column: "claimed_by", Value: claimant}).
		Select(
			"status", "attempt_count", "next_attempt_at", "claimed_by", "claimed_until",
			"close_done", "closed_direction", "closed_quantity", "close_average_price", "close_position_vanished",
			"open_done", "opened_quantity", "open_average_price", "opened_at",
			"stop_loss_price", "take_profit_price", "stop_loss_placed", "take_profit_placed", "protection_done",
			"outcome", "reason", "protection_missing", "settled_at").
		UpdateColumns(contractAutoOrder)
	if result.Error != nil {
		return false, fmt.Errorf("save contract auto order progress: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

func (contractAutoOrderRepository *ContractAutoOrderRepository) FindByBotRunNumbers(
	executionContext context.Context, strategyBotID uint, runNumbers []int,
) ([]entities.ContractAutoOrder, error) {
	contractAutoOrders := []entities.ContractAutoOrder{}
	if len(runNumbers) == 0 {
		return contractAutoOrders, nil
	}

	runNumberValues := make([]any, 0, len(runNumbers))
	for _, runNumber := range runNumbers {
		runNumberValues = append(runNumberValues, runNumber)
	}

	result := contractAutoOrderRepository.database.within(executionContext).
		Where(clause.Eq{Column: "strategy_bot_id", Value: strategyBotID}).
		Where(clause.IN{Column: "run_number", Values: runNumberValues}).
		Find(&contractAutoOrders)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract auto orders by round: %w", result.Error)
	}

	return contractAutoOrders, nil
}

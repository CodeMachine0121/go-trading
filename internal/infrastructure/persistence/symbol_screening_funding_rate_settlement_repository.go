package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SymbolScreeningFundingRateSettlementRepository struct {
	database *gorm.DB
}

func NewSymbolScreeningFundingRateSettlementRepository(
	database *gorm.DB,
) *SymbolScreeningFundingRateSettlementRepository {
	return &SymbolScreeningFundingRateSettlementRepository{database: database}
}

func (settlementRepository *SymbolScreeningFundingRateSettlementRepository) FindInRange(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) ([]entities.SymbolScreeningFundingRateSettlement, error) {
	settlements := []entities.SymbolScreeningFundingRateSettlement{}
	found := settlementRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Gte{Column: "settlement_time", Value: startTime.UTC()}).
		Where(clause.Lt{Column: "settlement_time", Value: endTime.UTC()}).
		Order("settlement_time ASC").
		Find(&settlements)
	if found.Error != nil {
		return nil, fmt.Errorf("find symbol screening funding rate settlements: %w", found.Error)
	}

	return settlements, nil
}

func (settlementRepository *SymbolScreeningFundingRateSettlementRepository) SaveAll(
	executionContext context.Context, settlements []entities.SymbolScreeningFundingRateSettlement,
) error {
	if len(settlements) == 0 {
		return nil
	}
	saved := settlementRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "settlement_time"}},
			DoUpdates: clause.AssignmentColumns([]string{"funding_rate", "mark_price", "fetched_at"}),
		}).
		CreateInBatches(&settlements, symbolScreeningSaveBatchSize)
	if saved.Error != nil {
		return fmt.Errorf("save symbol screening funding rate settlements: %w", saved.Error)
	}

	return nil
}

func (settlementRepository *SymbolScreeningFundingRateSettlementRepository) DeleteFetchedBefore(
	executionContext context.Context, cutoff time.Time,
) (int, error) {
	deleted := settlementRepository.database.WithContext(executionContext).
		Where(clause.Lt{Column: "fetched_at", Value: cutoff.UTC()}).
		Delete(&entities.SymbolScreeningFundingRateSettlement{})
	if deleted.Error != nil {
		return 0, fmt.Errorf("delete expired symbol screening funding rate settlements: %w", deleted.Error)
	}

	return int(deleted.RowsAffected), nil
}

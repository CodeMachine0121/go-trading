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

// ContractFundingRateSettlementRepository stores perpetual contract funding rate
// settlements in PostgreSQL.
type ContractFundingRateSettlementRepository struct {
	database *gorm.DB
}

func NewContractFundingRateSettlementRepository(database *gorm.DB) *ContractFundingRateSettlementRepository {
	return &ContractFundingRateSettlementRepository{database: database}
}

// fundingRateSettlementSaveBatchSize is how many settlements go into one statement.
// A contract listed years ago has thousands of them, and the count only grows, while
// PostgreSQL takes at most sixty-five thousand values in a single statement.
const fundingRateSettlementSaveBatchSize = 1000

// SaveAllIfAbsent stores every settlement nothing is held for yet, a batch at a time,
// and says how many it stored. An empty batch never reaches the store: the driver
// refuses a statement with no rows, and "nothing new this hour" is an ordinary answer.
func (settlementRepository *ContractFundingRateSettlementRepository) SaveAllIfAbsent(
	executionContext context.Context, settlements []entities.ContractFundingRateSettlement,
) (int, error) {
	if len(settlements) == 0 {
		return 0, nil
	}

	result := settlementRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "settlement_time"}},
			DoNothing: true,
		}).
		CreateInBatches(&settlements, fundingRateSettlementSaveBatchSize)
	if result.Error != nil {
		return 0, fmt.Errorf("save contract funding rate settlements: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}

// FindLatest is the most recent settlement held for the contract.
func (settlementRepository *ContractFundingRateSettlementRepository) FindLatest(
	executionContext context.Context, symbol string,
) (entities.ContractFundingRateSettlement, bool, error) {
	latestSettlement := entities.ContractFundingRateSettlement{}

	result := settlementRepository.database.WithContext(executionContext).
		Where(&entities.ContractFundingRateSettlement{Symbol: symbol}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "settlement_time"}, Desc: true}).
		First(&latestSettlement)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ContractFundingRateSettlement{}, false, nil
	}
	if result.Error != nil {
		return entities.ContractFundingRateSettlement{}, false,
			fmt.Errorf("find latest contract funding rate settlement: %w", result.Error)
	}

	return latestSettlement, true, nil
}

// FindLatestBefore is the most recent settlement held for the contract whose
// settlement time is strictly before the cut-off.
func (settlementRepository *ContractFundingRateSettlementRepository) FindLatestBefore(
	executionContext context.Context, symbol string, cutoffTime time.Time,
) (entities.ContractFundingRateSettlement, bool, error) {
	latestSettlement := entities.ContractFundingRateSettlement{}

	result := settlementRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: symbol},
			clause.Lt{Column: clause.Column{Name: "settlement_time"}, Value: cutoffTime},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "settlement_time"}, Desc: true}).
		First(&latestSettlement)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ContractFundingRateSettlement{}, false, nil
	}
	if result.Error != nil {
		return entities.ContractFundingRateSettlement{}, false,
			fmt.Errorf("find latest contract funding rate settlement before cutoff: %w", result.Error)
	}

	return latestSettlement, true, nil
}

// FindInRange returns the settlements inside the query's range, earliest first.
func (settlementRepository *ContractFundingRateSettlementRepository) FindInRange(
	executionContext context.Context, query domains.KCandleQueryDomain, limit int,
) ([]entities.ContractFundingRateSettlement, error) {
	settlements := make([]entities.ContractFundingRateSettlement, 0, min(limit, preallocationCeiling))

	result := settlementRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: query.Symbol()},
			clause.Gte{Column: clause.Column{Name: "settlement_time"}, Value: query.StartTime()},
			clause.Lte{Column: clause.Column{Name: "settlement_time"}, Value: query.EndTime()},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "settlement_time"}}).
		Limit(limit).
		Find(&settlements)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract funding rate settlements in range: %w", result.Error)
	}

	return settlements, nil
}

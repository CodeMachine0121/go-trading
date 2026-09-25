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

// ContractPositionStatisticRepository stores perpetual contract position statistics
// in PostgreSQL.
type ContractPositionStatisticRepository struct {
	database *gorm.DB
}

func NewContractPositionStatisticRepository(database *gorm.DB) *ContractPositionStatisticRepository {
	return &ContractPositionStatisticRepository{database: database}
}

// saveBatchSize is how many statistics go into one statement. A thirty-day catch-up
// is over eight thousand of them, and at ten figures each that is more than the
// sixty-five thousand values PostgreSQL takes in a single statement.
const positionStatisticSaveBatchSize = 1000

// SaveAllIfAbsent stores every statistic nothing is held for yet, a batch at a time,
// and says how many it stored. An empty batch never reaches the store: the driver
// refuses a statement with no rows, and "nothing new in five minutes" is ordinary.
func (statisticRepository *ContractPositionStatisticRepository) SaveAllIfAbsent(
	executionContext context.Context, statistics []entities.ContractPositionStatistic,
) (int, error) {
	if len(statistics) == 0 {
		return 0, nil
	}

	result := statisticRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "symbol"}, {Name: "statistic_time"}},
			DoNothing: true,
		}).
		CreateInBatches(&statistics, positionStatisticSaveBatchSize)
	if result.Error != nil {
		return 0, fmt.Errorf("save contract position statistics: %w", result.Error)
	}

	return int(result.RowsAffected), nil
}

// FindLatest is the most recent statistic held for the contract.
func (statisticRepository *ContractPositionStatisticRepository) FindLatest(
	executionContext context.Context, symbol string,
) (entities.ContractPositionStatistic, bool, error) {
	latestStatistic := entities.ContractPositionStatistic{}

	result := statisticRepository.database.WithContext(executionContext).
		Where(&entities.ContractPositionStatistic{Symbol: symbol}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "statistic_time"}, Desc: true}).
		First(&latestStatistic)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.ContractPositionStatistic{}, false, nil
	}
	if result.Error != nil {
		return entities.ContractPositionStatistic{}, false,
			fmt.Errorf("find latest contract position statistic: %w", result.Error)
	}

	return latestStatistic, true, nil
}

// CountInRange is how many statistics are held for the contract between the two
// times, both ends included.
func (statisticRepository *ContractPositionStatisticRepository) CountInRange(
	executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
) (int, error) {
	heldCount := int64(0)

	result := statisticRepository.database.WithContext(executionContext).
		Model(&entities.ContractPositionStatistic{}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Where(clause.Gte{Column: "statistic_time", Value: startTime.UTC()}).
		Where(clause.Lte{Column: "statistic_time", Value: endTime.UTC()}).
		Count(&heldCount)
	if result.Error != nil {
		return 0, fmt.Errorf("count contract position statistics: %w", result.Error)
	}

	return int(heldCount), nil
}

// FindInRange returns the statistics inside the query's range, earliest first.
func (statisticRepository *ContractPositionStatisticRepository) FindInRange(
	executionContext context.Context, query domains.KCandleQueryDomain, limit int,
) ([]entities.ContractPositionStatistic, error) {
	statistics := make([]entities.ContractPositionStatistic, 0, min(limit, preallocationCeiling))

	result := statisticRepository.database.WithContext(executionContext).
		Clauses(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "symbol"}, Value: query.Symbol()},
			clause.Gte{Column: clause.Column{Name: "statistic_time"}, Value: query.StartTime()},
			clause.Lte{Column: clause.Column{Name: "statistic_time"}, Value: query.EndTime()},
		}}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "statistic_time"}}).
		Limit(limit).
		Find(&statistics)
	if result.Error != nil {
		return nil, fmt.Errorf("find contract position statistics in range: %w", result.Error)
	}

	return statistics, nil
}

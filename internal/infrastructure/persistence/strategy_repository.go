package persistence

import (
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// strategyWritableColumns are the only columns a rewrite may touch. Naming them is
// what makes "the identifier and the time it was first saved never change" true:
// they are not on the list, so no update can reach them however the entity handed in
// was filled.
var strategyWritableColumns = []string{
	"name", "script", "result_type", "aggregation_interval", "candle_count",
}

// StrategyRepository stores saved strategies in PostgreSQL.
type StrategyRepository struct {
	database *gorm.DB
}

func NewStrategyRepository(database *gorm.DB) *StrategyRepository {
	return &StrategyRepository{database: database}
}

// Save stores a new strategy, letting the unique index on the name decide whether it
// may exist. Asking first and creating afterwards would let two requests arriving at
// once both find the name free.
func (strategyRepository *StrategyRepository) Save(
	strategy entities.Strategy,
) (entities.Strategy, error) {
	result := strategyRepository.database.Create(&strategy)
	if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return entities.Strategy{}, fmt.Errorf(
			"%w: 策略名稱「%s」已被使用", domains.ErrStrategyNameConflict, strategy.Name)
	}
	if result.Error != nil {
		return entities.Strategy{}, fmt.Errorf("save strategy: %w", result.Error)
	}

	return strategy, nil
}

// Update rewrites the five things a strategy remembers and hands back the strategy as
// it now stands. Only the writable columns are sent, so the identifier and the time
// the strategy was first saved are out of reach by construction.
func (strategyRepository *StrategyRepository) Update(
	strategy entities.Strategy,
) (entities.Strategy, error) {
	result := strategyRepository.database.
		Model(&entities.Strategy{}).
		Where("id = ?", strategy.ID).
		Select(strategyWritableColumns).
		Updates(strategy)

	if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
		return entities.Strategy{}, fmt.Errorf(
			"%w: 策略名稱「%s」已被使用", domains.ErrStrategyNameConflict, strategy.Name)
	}
	if result.Error != nil {
		return entities.Strategy{}, fmt.Errorf("update strategy: %w", result.Error)
	}

	// Reading it back is what reports a strategy that is not there: nothing was
	// rewritten, so nothing can be found. Checking the rows written first would ask
	// the same question twice and answer it identically.
	return strategyRepository.FindOne(strategy.ID)
}

// FindOne returns the strategy carrying this identifier.
func (strategyRepository *StrategyRepository) FindOne(id uint) (entities.Strategy, error) {
	strategy := entities.Strategy{}

	result := strategyRepository.database.First(&strategy, "id = ?", id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.Strategy{}, domains.ErrStrategyNotFound
	}
	if result.Error != nil {
		return entities.Strategy{}, fmt.Errorf("find strategy: %w", result.Error)
	}

	return strategy, nil
}

// FindAll returns every saved strategy, ordered by name.
func (strategyRepository *StrategyRepository) FindAll() ([]entities.Strategy, error) {
	strategies := make([]entities.Strategy, 0)

	result := strategyRepository.database.
		Order(clause.OrderByColumn{Column: clause.Column{Name: "name"}}).
		Find(&strategies)
	if result.Error != nil {
		return nil, fmt.Errorf("find strategies: %w", result.Error)
	}

	return strategies, nil
}

// Delete removes the strategy for good. There is no keeping of what was deleted:
// a name that is still held by something nobody can read is a name nobody can
// explain, and this is a single person's own collection.
func (strategyRepository *StrategyRepository) Delete(id uint) error {
	result := strategyRepository.database.Delete(&entities.Strategy{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete strategy: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domains.ErrStrategyNotFound
	}

	return nil
}

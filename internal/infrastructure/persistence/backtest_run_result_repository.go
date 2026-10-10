package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BacktestRunResultRepository struct {
	database ambientTransactionDatabase
}

func NewBacktestRunResultRepository(database *gorm.DB) *BacktestRunResultRepository {
	return &BacktestRunResultRepository{database: ambientTransactionDatabase{root: database}}
}

func (backtestRunResultRepository *BacktestRunResultRepository) Save(
	executionContext context.Context, result entities.BacktestRunResult,
) error {
	saved := backtestRunResultRepository.database.within(executionContext).
		Clauses(clause.OnConflict{UpdateAll: true}).
		Create(&result)
	if saved.Error != nil {
		return fmt.Errorf("save backtest run result: %w", saved.Error)
	}

	return nil
}

func (backtestRunResultRepository *BacktestRunResultRepository) FindOne(
	executionContext context.Context, backtestRunID uint,
) (entities.BacktestRunResult, bool, error) {
	result := entities.BacktestRunResult{}
	found := backtestRunResultRepository.database.within(executionContext).
		Where(clause.Eq{Column: "backtest_run_id", Value: backtestRunID}).
		First(&result)
	if errors.Is(found.Error, gorm.ErrRecordNotFound) {
		return entities.BacktestRunResult{}, false, nil
	}
	if found.Error != nil {
		return entities.BacktestRunResult{}, false, fmt.Errorf("find backtest run result: %w", found.Error)
	}

	return result, true, nil
}

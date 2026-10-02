package persistence

import (
	"context"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LiveKCandleSnapshotRepository struct {
	database *gorm.DB
}

func NewLiveKCandleSnapshotRepository(database *gorm.DB) *LiveKCandleSnapshotRepository {
	return &LiveKCandleSnapshotRepository{database: database}
}

func (liveKCandleSnapshotRepository *LiveKCandleSnapshotRepository) Save(
	executionContext context.Context, snapshot entities.LiveKCandleSnapshot,
) error {
	result := liveKCandleSnapshotRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "symbol"}}, UpdateAll: true}).
		Create(&snapshot)
	if result.Error != nil {
		return fmt.Errorf("save live k candle snapshot: %w", result.Error)
	}

	return nil
}

func (liveKCandleSnapshotRepository *LiveKCandleSnapshotRepository) FindBySymbols(
	executionContext context.Context, symbols []string,
) ([]entities.LiveKCandleSnapshot, error) {
	snapshots := []entities.LiveKCandleSnapshot{}
	if len(symbols) == 0 {
		return snapshots, nil
	}

	// GORM's IN clause takes its values loosely typed, as elsewhere in this package.
	symbolValues := make([]any, 0, len(symbols))
	for _, symbol := range symbols {
		symbolValues = append(symbolValues, symbol)
	}

	result := liveKCandleSnapshotRepository.database.WithContext(executionContext).
		Where(clause.IN{Column: "symbol", Values: symbolValues}).
		Find(&snapshots)
	if result.Error != nil {
		return nil, fmt.Errorf("find live k candle snapshots: %w", result.Error)
	}

	return snapshots, nil
}

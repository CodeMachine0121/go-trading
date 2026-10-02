package persistence

import (
	"context"
	"fmt"
	"time"

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
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "symbol"}, {Name: "open_time"}}, UpdateAll: true,
		}).
		Create(&snapshot)
	if result.Error != nil {
		return fmt.Errorf("save live k candle snapshot: %w", result.Error)
	}

	return nil
}

func (liveKCandleSnapshotRepository *LiveKCandleSnapshotRepository) FindObservedAfter(
	executionContext context.Context, symbols []string, since time.Time,
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
		Where(clause.Gt{Column: "observed_at", Value: since.UTC()}).
		Order("open_time ASC").
		Order("observed_at ASC").
		Find(&snapshots)
	if result.Error != nil {
		return nil, fmt.Errorf("find live k candle snapshots: %w", result.Error)
	}

	return snapshots, nil
}

func (liveKCandleSnapshotRepository *LiveKCandleSnapshotRepository) DeleteObservedBefore(
	executionContext context.Context, cutoff time.Time,
) error {
	result := liveKCandleSnapshotRepository.database.WithContext(executionContext).
		Where(clause.Lt{Column: "observed_at", Value: cutoff.UTC()}).
		Delete(&entities.LiveKCandleSnapshot{})
	if result.Error != nil {
		return fmt.Errorf("delete old live k candle snapshots: %w", result.Error)
	}

	return nil
}

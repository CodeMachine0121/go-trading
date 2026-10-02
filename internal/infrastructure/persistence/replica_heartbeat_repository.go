package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReplicaHeartbeatRepository struct {
	database *gorm.DB
}

func NewReplicaHeartbeatRepository(database *gorm.DB) *ReplicaHeartbeatRepository {
	return &ReplicaHeartbeatRepository{database: database}
}

func (replicaHeartbeatRepository *ReplicaHeartbeatRepository) Beat(
	executionContext context.Context, replicaName string, seenAt time.Time,
) error {
	result := replicaHeartbeatRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_seen_at"}),
		}).
		Create(&entities.ReplicaHeartbeat{Name: replicaName, LastSeenAt: seenAt.UTC()})
	if result.Error != nil {
		return fmt.Errorf("record replica heartbeat: %w", result.Error)
	}

	return nil
}

func (replicaHeartbeatRepository *ReplicaHeartbeatRepository) FindSeenSince(
	executionContext context.Context, since time.Time,
) ([]string, error) {
	replicaNames := []string{}

	result := replicaHeartbeatRepository.database.WithContext(executionContext).
		Model(&entities.ReplicaHeartbeat{}).
		Where(clause.Gte{Column: "last_seen_at", Value: since.UTC()}).
		Pluck("name", &replicaNames)
	if result.Error != nil {
		return nil, fmt.Errorf("find replicas seen recently: %w", result.Error)
	}

	return replicaNames, nil
}

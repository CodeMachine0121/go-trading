package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// releasedLeaseExpiry is far enough in the past that any replica's clock reads the lease as expired.
var releasedLeaseExpiry = time.Unix(0, 0).UTC()

type JobLeadershipLeaseRepository struct {
	database *gorm.DB
}

func NewJobLeadershipLeaseRepository(database *gorm.DB) *JobLeadershipLeaseRepository {
	return &JobLeadershipLeaseRepository{database: database}
}

// Acquire is a single upsert whose update only applies to an expired lease or our own, so the row lock decides a race.
func (jobLeadershipLeaseRepository *JobLeadershipLeaseRepository) Acquire(
	executionContext context.Context, name string, holderName string, now time.Time, expiresAt time.Time,
) (bool, error) {
	lease := entities.JobLeadershipLease{
		Name: name, HolderName: holderName, ExpiresAt: expiresAt.UTC(), UpdatedAt: now.UTC(),
	}

	result := jobLeadershipLeaseRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"holder_name", "expires_at", "updated_at"}),
			Where: clause.Where{Exprs: []clause.Expression{clause.Or(
				clause.Eq{
					Column: clause.Column{Table: lease.TableName(), Name: "holder_name"}, Value: holderName,
				},
				clause.Lt{
					Column: clause.Column{Table: lease.TableName(), Name: "expires_at"}, Value: now.UTC(),
				},
			)}},
		}).
		Create(&lease)
	if result.Error != nil {
		return false, fmt.Errorf("acquire job leadership lease: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

func (jobLeadershipLeaseRepository *JobLeadershipLeaseRepository) Release(
	executionContext context.Context, name string, holderName string,
) error {
	result := jobLeadershipLeaseRepository.database.WithContext(executionContext).
		Model(&entities.JobLeadershipLease{}).
		Where(clause.Eq{Column: "name", Value: name}).
		Where(clause.Eq{Column: "holder_name", Value: holderName}).
		Update("expires_at", releasedLeaseExpiry)
	if result.Error != nil {
		return fmt.Errorf("release job leadership lease: %w", result.Error)
	}

	return nil
}

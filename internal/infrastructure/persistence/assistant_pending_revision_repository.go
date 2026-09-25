package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssistantPendingRevisionRepository struct {
	database *gorm.DB
}

func NewAssistantPendingRevisionRepository(database *gorm.DB) *AssistantPendingRevisionRepository {
	return &AssistantPendingRevisionRepository{database: database}
}

func (assistantPendingRevisionRepository *AssistantPendingRevisionRepository) Save(
	executionContext context.Context, assistantPendingRevision entities.AssistantPendingRevision,
) (entities.AssistantPendingRevision, error) {
	result := assistantPendingRevisionRepository.database.WithContext(executionContext).
		Create(&assistantPendingRevision)
	if result.Error != nil {
		return entities.AssistantPendingRevision{}, fmt.Errorf("save assistant pending revision: %w", result.Error)
	}

	return assistantPendingRevision, nil
}

func (assistantPendingRevisionRepository *AssistantPendingRevisionRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.AssistantPendingRevision, error) {
	assistantPendingRevision := entities.AssistantPendingRevision{}

	result := assistantPendingRevisionRepository.database.WithContext(executionContext).
		First(&assistantPendingRevision, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.AssistantPendingRevision{}, domains.AssistantPendingRevisionNotFound(id)
	}
	if result.Error != nil {
		return entities.AssistantPendingRevision{}, fmt.Errorf("find assistant pending revision: %w", result.Error)
	}

	return assistantPendingRevision, nil
}

// TransitionStatus is a single conditional update, so only one of two simultaneous presses can win it.
func (assistantPendingRevisionRepository *AssistantPendingRevisionRepository) TransitionStatus(
	executionContext context.Context, id uint, from string, to string,
) (bool, error) {
	result := assistantPendingRevisionRepository.database.WithContext(executionContext).
		Model(&entities.AssistantPendingRevision{}).
		Where(clause.Eq{Column: "id", Value: id}).
		Where(clause.Eq{Column: "status", Value: from}).
		Update("status", to)
	if result.Error != nil {
		return false, fmt.Errorf("transition assistant pending revision: %w", result.Error)
	}

	return result.RowsAffected == 1, nil
}

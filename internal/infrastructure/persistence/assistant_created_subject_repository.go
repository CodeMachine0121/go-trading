package persistence

import (
	"context"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AssistantCreatedSubjectRepository struct {
	database *gorm.DB
}

func NewAssistantCreatedSubjectRepository(database *gorm.DB) *AssistantCreatedSubjectRepository {
	return &AssistantCreatedSubjectRepository{database: database}
}

func (assistantCreatedSubjectRepository *AssistantCreatedSubjectRepository) Save(
	executionContext context.Context, assistantCreatedSubject entities.AssistantCreatedSubject,
) error {
	result := assistantCreatedSubjectRepository.database.WithContext(executionContext).
		Clauses(clause.OnConflict{DoNothing: true}).
		Omit("Conversation").
		Create(&assistantCreatedSubject)
	if result.Error != nil {
		return fmt.Errorf("save assistant created subject: %w", result.Error)
	}

	return nil
}

func (assistantCreatedSubjectRepository *AssistantCreatedSubjectRepository) Exists(
	executionContext context.Context, conversationID uint, subjectKind string, subjectID uint,
) (bool, error) {
	count := int64(0)

	result := assistantCreatedSubjectRepository.database.WithContext(executionContext).
		Model(&entities.AssistantCreatedSubject{}).
		Where(clause.Eq{Column: "conversation_id", Value: conversationID}).
		Where(clause.Eq{Column: "subject_kind", Value: subjectKind}).
		Where(clause.Eq{Column: "subject_id", Value: subjectID}).
		Count(&count)
	if result.Error != nil {
		return false, fmt.Errorf("find assistant created subject: %w", result.Error)
	}

	return count > 0, nil
}

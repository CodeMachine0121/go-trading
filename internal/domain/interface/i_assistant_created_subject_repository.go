package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_assistant_created_subject_repository.go -destination=mocks/mock_i_assistant_created_subject_repository.go -package=mocks

// IAssistantCreatedSubjectRepository remembers what the assistant created in which conversation.
type IAssistantCreatedSubjectRepository interface {
	// Save ignores a subject already remembered for the conversation.
	Save(executionContext context.Context, assistantCreatedSubject entities.AssistantCreatedSubject) error
	Exists(executionContext context.Context, conversationID uint, subjectKind string, subjectID uint) (bool, error)
}

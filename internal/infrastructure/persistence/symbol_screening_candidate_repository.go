package persistence

import (
	"context"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SymbolScreeningCandidateRepository struct {
	database ambientTransactionDatabase
}

func NewSymbolScreeningCandidateRepository(database *gorm.DB) *SymbolScreeningCandidateRepository {
	return &SymbolScreeningCandidateRepository{database: ambientTransactionDatabase{root: database}}
}

func (candidateRepository *SymbolScreeningCandidateRepository) FindByRun(
	executionContext context.Context, runID uint,
) ([]entities.SymbolScreeningCandidate, error) {
	candidates := []entities.SymbolScreeningCandidate{}
	listed := candidateRepository.database.within(executionContext).
		Where(clause.Eq{Column: "run_id", Value: runID}).
		Order("position ASC").
		Find(&candidates)
	if listed.Error != nil {
		return nil, fmt.Errorf("list symbol screening candidates: %w", listed.Error)
	}

	return candidates, nil
}

func (candidateRepository *SymbolScreeningCandidateRepository) Save(
	executionContext context.Context, candidate entities.SymbolScreeningCandidate,
) error {
	saved := candidateRepository.database.within(executionContext).Select("*").Save(&candidate)
	if saved.Error != nil {
		return fmt.Errorf("save symbol screening candidate: %w", saved.Error)
	}

	return nil
}

func (candidateRepository *SymbolScreeningCandidateRepository) CancelUnfinished(
	executionContext context.Context, runID uint,
) error {
	cancelled := candidateRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningCandidate{}).
		Where(clause.Eq{Column: "run_id", Value: runID}).
		Where(clause.IN{Column: "status", Values: []any{
			string(vo.SymbolScreeningCandidateWaiting), string(vo.SymbolScreeningCandidateProcessing),
		}}).
		Update("status", string(vo.SymbolScreeningCandidateCancelled))
	if cancelled.Error != nil {
		return fmt.Errorf("cancel unfinished symbol screening candidates: %w", cancelled.Error)
	}

	return nil
}

func (candidateRepository *SymbolScreeningCandidateRepository) CancelUnfinishedOfRuns(
	executionContext context.Context, runIDs []uint,
) error {
	if len(runIDs) == 0 {
		return nil
	}
	runValues := make([]any, 0, len(runIDs))
	for _, runID := range runIDs {
		runValues = append(runValues, runID)
	}
	cancelled := candidateRepository.database.within(executionContext).
		Model(&entities.SymbolScreeningCandidate{}).
		Where(clause.IN{Column: "run_id", Values: runValues}).
		Where(clause.IN{Column: "status", Values: []any{
			string(vo.SymbolScreeningCandidateWaiting), string(vo.SymbolScreeningCandidateProcessing),
		}}).
		Update("status", string(vo.SymbolScreeningCandidateCancelled))
	if cancelled.Error != nil {
		return fmt.Errorf("cancel unfinished candidates of runs: %w", cancelled.Error)
	}

	return nil
}

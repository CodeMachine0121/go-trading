package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_symbol_screening_candidate_repository.go -destination=mocks/mock_i_symbol_screening_candidate_repository.go -package=mocks

type ISymbolScreeningCandidateRepository interface {
	FindByRun(executionContext context.Context, runID uint) ([]entities.SymbolScreeningCandidate, error)
	Save(executionContext context.Context, candidate entities.SymbolScreeningCandidate) error
	// CancelUnfinished marks every candidate still waiting or being screened as cancelled.
	CancelUnfinished(executionContext context.Context, runID uint) error
	CancelUnfinishedOfRuns(executionContext context.Context, runIDs []uint) error
}

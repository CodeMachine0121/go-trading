package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_symbol_screening_funding_rate_settlement_repository.go -destination=mocks/mock_i_symbol_screening_funding_rate_settlement_repository.go -package=mocks

type ISymbolScreeningFundingRateSettlementRepository interface {
	// FindInRange includes the start and excludes the end, oldest first.
	FindInRange(
		executionContext context.Context, symbol string, startTime time.Time, endTime time.Time,
	) ([]entities.SymbolScreeningFundingRateSettlement, error)
	SaveAll(executionContext context.Context, settlements []entities.SymbolScreeningFundingRateSettlement) error
	DeleteFetchedBefore(executionContext context.Context, cutoff time.Time) (int, error)
}

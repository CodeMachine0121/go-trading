package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_funding_rate_settlement_repository.go -destination=mocks/mock_i_contract_funding_rate_settlement_repository.go -package=mocks

type IContractFundingRateSettlementRepository interface {
	// SaveAllIfAbsent returns how many were stored; an already-held settlement is left unchanged because the venue never revises one.
	SaveAllIfAbsent(
		executionContext context.Context, settlements []entities.ContractFundingRateSettlement,
	) (int, error)
	FindLatest(
		executionContext context.Context, symbol string,
	) (entities.ContractFundingRateSettlement, bool, error)
	// FindLatestBefore returns the latest settlement strictly before cutoffTime, i.e. the rate already in force when a stretch of bars opens.
	FindLatestBefore(
		executionContext context.Context, symbol string, cutoffTime time.Time,
	) (entities.ContractFundingRateSettlement, bool, error)
	// FindInRange includes both ends, earliest first, at most limit.
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.ContractFundingRateSettlement, error)
}

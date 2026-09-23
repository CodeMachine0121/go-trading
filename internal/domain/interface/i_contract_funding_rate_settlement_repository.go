package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_funding_rate_settlement_repository.go -destination=mocks/mock_i_contract_funding_rate_settlement_repository.go -package=mocks

// IContractFundingRateSettlementRepository stores perpetual contract funding rate
// settlements.
type IContractFundingRateSettlementRepository interface {
	// SaveAllIfAbsent stores every settlement nothing is held for yet and says how
	// many it stored. A settlement already held is left exactly as it was: the venue
	// does not revise a settlement, so a second answer for the same one is the same
	// fact asked about twice.
	SaveAllIfAbsent(
		executionContext context.Context, settlements []entities.ContractFundingRateSettlement,
	) (int, error)
	// FindLatest is the most recent settlement held for the contract, and whether
	// there is one at all.
	FindLatest(
		executionContext context.Context, symbol string,
	) (entities.ContractFundingRateSettlement, bool, error)
	// FindLatestBefore is the most recent settlement held for the contract whose
	// settlement time is STRICTLY BEFORE cutoffTime, and whether there is one at all.
	// It is how a stretch of bars learns the rate that was already in force when its
	// first bar opened, however long before that the settlement took place.
	FindLatestBefore(
		executionContext context.Context, symbol string, cutoffTime time.Time,
	) (entities.ContractFundingRateSettlement, bool, error)
	// FindInRange returns the settlements whose settlement time falls inside the
	// query's range, both ends included, earliest first, at most limit of them.
	FindInRange(
		executionContext context.Context, query domains.KCandleQueryDomain, limit int,
	) ([]entities.ContractFundingRateSettlement, error)
}

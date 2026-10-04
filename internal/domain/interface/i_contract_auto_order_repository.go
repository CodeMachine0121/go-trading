package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_contract_auto_order_repository.go -destination=mocks/mock_i_contract_auto_order_repository.go -package=mocks

// IContractAutoOrderRepository holds auto orders waiting to be carried out; every write after Enqueue is guarded by who holds the order, so two replicas can never both carry one out.
type IContractAutoOrderRepository interface {
	// Enqueue takes part in a surrounding transaction; a second order for the same round is silently dropped.
	Enqueue(executionContext context.Context, contractAutoOrder entities.ContractAutoOrder) error

	// FindDispatchCandidates returns, oldest first and at most limit, each bot's oldest unsettled order, so one bot's orders run one after another.
	FindDispatchCandidates(executionContext context.Context, limit int) ([]entities.ContractAutoOrder, error)

	// Claim takes the order for claimant when it is ready and due at moment, or its holder's claim ran out by moment; false means another replica has it.
	Claim(
		executionContext context.Context, id uint, claimant string, moment time.Time, claimedUntil time.Time,
	) (bool, error)

	// SaveProgress writes everything the order has done and become, takes part in a surrounding transaction, and reports false when claimant no longer holds it.
	SaveProgress(
		executionContext context.Context, contractAutoOrder entities.ContractAutoOrder, claimant string,
	) (bool, error)

	// FindByBotRunNumbers returns the bot's orders queued by the given rounds.
	FindByBotRunNumbers(
		executionContext context.Context, strategyBotID uint, runNumbers []int,
	) ([]entities.ContractAutoOrder, error)
}

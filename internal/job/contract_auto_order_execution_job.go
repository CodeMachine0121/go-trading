package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractAutoOrderExecutionJob carries out queued auto orders every few seconds on every replica; an order cut off mid-way stays claimed and is resumed once the claim runs out.
type ContractAutoOrderExecutionJob struct {
	*repeatingRound
}

func NewContractAutoOrderExecutionJob(
	contractAutoOrderExecutionApplication *application.ContractAutoOrderExecutionApplication, interval time.Duration,
) *ContractAutoOrderExecutionJob {
	return &ContractAutoOrderExecutionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if _, executeError := contractAutoOrderExecutionApplication.ExecuteDueAutoOrders(
				executionContext); executeError != nil {
				log.Printf("contract auto orders could not be carried out this round: %v", executeError)
			}
		})}
}

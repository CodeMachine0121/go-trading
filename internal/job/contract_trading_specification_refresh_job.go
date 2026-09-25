package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractTradingSpecificationRefreshJob refreshes trading specifications on start and every interval.
type ContractTradingSpecificationRefreshJob struct {
	*repeatingRound
}

func NewContractTradingSpecificationRefreshJob(
	contractTradingSymbolApplication *application.ContractTradingSymbolApplication,
	interval time.Duration,
) *ContractTradingSpecificationRefreshJob {
	return &ContractTradingSpecificationRefreshJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			refreshedCount, refreshError := contractTradingSymbolApplication.
				RefreshTradingSpecifications(executionContext)
			if refreshError != nil {
				log.Printf("contract trading specification refresh did not run: %v", refreshError)

				return
			}

			log.Printf("contract trading specification refresh updated %d contracts", refreshedCount)
		})}
}

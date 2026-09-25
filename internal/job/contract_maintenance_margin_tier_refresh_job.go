package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractMaintenanceMarginTierRefreshJob refreshes maintenance margin tiers on start and every interval; it is only assembled when an account is configured.
type ContractMaintenanceMarginTierRefreshJob struct {
	*repeatingRound
}

func NewContractMaintenanceMarginTierRefreshJob(
	tierApplication *application.ContractMaintenanceMarginTierApplication,
	interval time.Duration,
) *ContractMaintenanceMarginTierRefreshJob {
	return &ContractMaintenanceMarginTierRefreshJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			refreshReport, refreshError := tierApplication.RefreshLadders(executionContext)
			if refreshError != nil {
				log.Printf("contract maintenance margin refresh did not run: %v", refreshError)

				return
			}

			for _, refusedLadder := range refreshReport.RefusedLadders {
				log.Printf("contract maintenance margin refresh kept %s as it was: %s",
					refusedLadder.Symbol, refusedLadder.Reason)
			}
			log.Printf("contract maintenance margin refresh updated %d contracts", refreshReport.RefreshedCount)
		})}
}

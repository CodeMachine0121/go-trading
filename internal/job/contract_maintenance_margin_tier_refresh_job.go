package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractMaintenanceMarginTierRefreshJob keeps every known perpetual contract's full
// maintenance margin ladder current: once on start, then every interval.
//
// It is only ever assembled when an account is configured — without one there is
// nothing it could ask, and a line every day saying so would be noise.
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

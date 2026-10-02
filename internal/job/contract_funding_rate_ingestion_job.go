package job

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractFundingRateIngestionJob syncs funding rate settlements on start and every interval, separate from the candle job because settlements happen only a few times a day.
// It works only while this replica is on duty.
type ContractFundingRateIngestionJob struct {
	*repeatingRound
}

func NewContractFundingRateIngestionJob(
	contractFundingRateApplication *application.ContractFundingRateApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *ContractFundingRateIngestionJob {
	roundReporter := contractSeriesRoundReporter{seriesName: "funding rate"}

	return &ContractFundingRateIngestionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				return
			}

			roundReporter.report(contractFundingRateApplication.RunRound(executionContext))
		})}
}

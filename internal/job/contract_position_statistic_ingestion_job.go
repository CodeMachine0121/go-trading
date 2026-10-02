package job

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractPositionStatisticIngestionJob records position statistics on start and every interval; the venue keeps only thirty days, so missed days are lost for good.
// It works only while this replica is on duty.
type ContractPositionStatisticIngestionJob struct {
	*repeatingRound
}

func NewContractPositionStatisticIngestionJob(
	positionStatisticApplication *application.ContractPositionStatisticApplication,
	jobLeadershipApplication *application.JobLeadershipApplication,
	interval time.Duration,
) *ContractPositionStatisticIngestionJob {
	roundReporter := contractSeriesRoundReporter{seriesName: "position statistic"}

	return &ContractPositionStatisticIngestionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			if !jobLeadershipApplication.IsLeader() {
				return
			}

			roundReporter.report(positionStatisticApplication.RunRound(executionContext))
		})}
}

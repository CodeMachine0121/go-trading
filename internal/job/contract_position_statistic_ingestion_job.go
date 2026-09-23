package job

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractPositionStatisticIngestionJob keeps recording every watched perpetual
// contract's position statistics: once on start, which records whatever the venue
// still remembers of the time the system was down, and then every interval.
//
// It is the job whose absence costs the most. The venue keeps thirty days of these,
// so a week of this job not running is a week that is gone for good.
type ContractPositionStatisticIngestionJob struct {
	*repeatingRound
}

func NewContractPositionStatisticIngestionJob(
	positionStatisticApplication *application.ContractPositionStatisticApplication,
	interval time.Duration,
) *ContractPositionStatisticIngestionJob {
	roundReporter := contractSeriesRoundReporter{seriesName: "position statistic"}

	return &ContractPositionStatisticIngestionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			roundReporter.report(positionStatisticApplication.RunRound(executionContext))
		})}
}

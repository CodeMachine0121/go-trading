package job

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractFundingRateIngestionJob keeps every watched perpetual contract's funding
// rate settlements complete: once on start, which catches up whatever settled while
// the system was down, and then every interval.
//
// It runs apart from the candle job because the two keep different time. A contract
// settles a few times a day, so asking every minute would spend the venue's allowance
// on answers that are almost always empty.
type ContractFundingRateIngestionJob struct {
	*repeatingRound
}

func NewContractFundingRateIngestionJob(
	contractFundingRateApplication *application.ContractFundingRateApplication,
	interval time.Duration,
) *ContractFundingRateIngestionJob {
	roundReporter := contractSeriesRoundReporter{seriesName: "funding rate"}

	return &ContractFundingRateIngestionJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			roundReporter.report(contractFundingRateApplication.RunRound(executionContext))
		})}
}

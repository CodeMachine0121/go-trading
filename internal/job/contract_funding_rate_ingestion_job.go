package job

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// ContractFundingRateIngestionJob syncs funding rate settlements on start and every interval, separate from the candle job because settlements happen only a few times a day.
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

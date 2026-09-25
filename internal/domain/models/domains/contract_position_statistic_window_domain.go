package domains

import "time"

// contractPositionStatisticRetention is how far back the venue answers; older requests are refused outright rather than answered empty.
const contractPositionStatisticRetention = 30 * 24 * time.Hour

// contractPositionStatisticRecheckSpan re-asks behind the last held statistic to catch moments skipped while an answer was pending; it still fits one page.
const contractPositionStatisticRecheckSpan = time.Hour

// ContractPositionStatisticWindowDomain is one round's request range for one contract, never starting before what the venue still retains.
type ContractPositionStatisticWindowDomain struct {
	startTime time.Time
	endTime   time.Time
}

// NewContractPositionStatisticWindowDomain settles the window against one clock reading; hasLatest says whether latestHeld is meaningful.
func NewContractPositionStatisticWindowDomain(
	currentTime time.Time, latestHeld time.Time, hasLatest bool,
) ContractPositionStatisticWindowDomain {
	// Two grid steps inside the retention edge, because the edge keeps moving and the venue refuses anything past it.
	earliestAnswerable := currentTime.Add(-contractPositionStatisticRetention).
		Truncate(ContractPositionStatisticInterval).Add(2 * ContractPositionStatisticInterval)

	startTime := earliestAnswerable
	if hasLatest {
		startTime = latestHeld.UTC().Add(ContractPositionStatisticInterval).
			Add(-contractPositionStatisticRecheckSpan)
		if startTime.Before(earliestAnswerable) {
			startTime = earliestAnswerable
		}
	}

	return ContractPositionStatisticWindowDomain{
		startTime: startTime.UTC(),
		endTime:   currentTime.UTC().Truncate(ContractPositionStatisticInterval),
	}
}

func (windowDomain ContractPositionStatisticWindowDomain) StartTime() time.Time {
	return windowDomain.startTime
}

// EndTime is the latest statistic time that can exist yet.
func (windowDomain ContractPositionStatisticWindowDomain) EndTime() time.Time {
	return windowDomain.endTime
}

func (windowDomain ContractPositionStatisticWindowDomain) IsEmpty() bool {
	return windowDomain.startTime.After(windowDomain.endTime)
}

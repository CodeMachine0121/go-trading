package domains

import "time"

// contractPositionStatisticRetention is how far back the venue will answer about
// position statistics at all. Asked about anything older, it refuses the question
// rather than answering with nothing.
const contractPositionStatisticRetention = 30 * 24 * time.Hour

// ContractPositionStatisticWindowDomain is the stretch one round asks the venue about
// for one contract: from just after the last statistic held to the latest one that
// can exist.
//
// **The start never reaches past what the venue keeps.** A contract with nothing held,
// or one whose last statistic is older than thirty days, starts at the first moment
// the venue can still answer for. What lies between is simply gone — the venue's
// limit, not a failure of this round — which is why these have to be recorded while
// they can be.
type ContractPositionStatisticWindowDomain struct {
	startTime time.Time
	endTime   time.Time
}

// NewContractPositionStatisticWindowDomain settles the window against one reading of
// the clock. hasLatest says whether latestHeld is a statistic at all.
func NewContractPositionStatisticWindowDomain(
	currentTime time.Time, latestHeld time.Time, hasLatest bool,
) ContractPositionStatisticWindowDomain {
	// The first grid moment strictly inside the venue's thirty days. A moment exactly
	// thirty days old is at the edge of what the venue accepts, and by the time the
	// question reaches it the edge has moved on.
	earliestAnswerable := currentTime.Add(-contractPositionStatisticRetention).
		Truncate(ContractPositionStatisticInterval).Add(ContractPositionStatisticInterval)

	startTime := earliestAnswerable
	if hasLatest {
		startTime = latestHeld.UTC().Add(ContractPositionStatisticInterval)
		if startTime.Before(earliestAnswerable) {
			startTime = earliestAnswerable
		}
	}

	return ContractPositionStatisticWindowDomain{
		startTime: startTime.UTC(),
		endTime:   currentTime.UTC().Truncate(ContractPositionStatisticInterval),
	}
}

// StartTime is the first statistic time to ask about.
func (windowDomain ContractPositionStatisticWindowDomain) StartTime() time.Time {
	return windowDomain.startTime
}

// EndTime is the latest statistic time that can exist yet.
func (windowDomain ContractPositionStatisticWindowDomain) EndTime() time.Time {
	return windowDomain.endTime
}

// IsEmpty says whether there is nothing new that could be asked about.
func (windowDomain ContractPositionStatisticWindowDomain) IsEmpty() bool {
	return windowDomain.startTime.After(windowDomain.endTime)
}

package domains

import "time"

// contractPositionStatisticRetention is how far back the venue will answer about
// position statistics at all. Asked about anything older, it refuses the question
// rather than answering with nothing.
const contractPositionStatisticRetention = 30 * 24 * time.Hour

// contractPositionStatisticRecheckSpan is how far behind the last statistic held
// every round asks again. A moment skipped because one of its three answers had not
// arrived yet sits behind statistics that were stored after it, so asking only from
// the last one held would never reach it again. An hour of re-asking still fits in a
// single page, so it costs no extra question — and what is already held is left as
// it was when it comes back.
const contractPositionStatisticRecheckSpan = time.Hour

// ContractPositionStatisticWindowDomain is the stretch one round asks the venue about
// for one contract: from an hour before the last statistic held to the latest one
// that can exist.
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
	// A full grid step inside the venue's thirty days, past the first one. The edge
	// keeps moving while a round waits its turn and walks its first stretch, so a
	// start one second inside it can be outside it by the time it is asked about —
	// and the venue refuses that question outright. Rounding down and stepping twice
	// leaves between five and ten minutes whatever the clock reads, which is the
	// oldest few minutes of thirty days given up for a question that is never refused.
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

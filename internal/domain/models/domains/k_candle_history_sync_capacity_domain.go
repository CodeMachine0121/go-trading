package domains

import (
	"errors"
	"fmt"
)

// ErrKCandleHistorySyncCapacityReached refuses rather than queues a sync while the limit is full, since a queued sync would outlive the request with nobody able to see it waiting.
var ErrKCandleHistorySyncCapacityReached = errors.New("k candle history sync capacity reached")

// KCandleHistorySyncCapacityDomain caps how many history syncs of one kind may run at once, since every running sync draws on the same venue allowance as scheduled ingestion.
type KCandleHistorySyncCapacityDomain struct {
	maximumConcurrentSyncs int
}

// NewKCandleHistorySyncCapacityDomain raises a non-positive limit to one so a bad setting cannot disable history sync altogether.
func NewKCandleHistorySyncCapacityDomain(maximumConcurrentSyncs int) KCandleHistorySyncCapacityDomain {
	return KCandleHistorySyncCapacityDomain{maximumConcurrentSyncs: max(maximumConcurrentSyncs, 1)}
}

func (kCandleHistorySyncCapacityDomain KCandleHistorySyncCapacityDomain) Admit(runningCount int) error {
	if runningCount >= kCandleHistorySyncCapacityDomain.maximumConcurrentSyncs {
		return fmt.Errorf("%w: 同時最多 %d 趟歷史同步，等其中一趟結束再開",
			ErrKCandleHistorySyncCapacityReached,
			kCandleHistorySyncCapacityDomain.maximumConcurrentSyncs)
	}

	return nil
}

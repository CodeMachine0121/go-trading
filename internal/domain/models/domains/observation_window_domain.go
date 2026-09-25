package domains

import (
	"fmt"
	"time"
)

// ObservationWindowDomain is a settled, ordered stretch no later than now; it knows nothing about candles or markets, so backtests can reuse it.
type ObservationWindowDomain struct {
	startTime time.Time
	endTime   time.Time
}

// NewObservationWindowDomain clamps a missing or future end to now, and refuses a missing start or a start not strictly before the end.
func NewObservationWindowDomain(
	declaredStartTime time.Time, declaredEndTime time.Time, now time.Time,
) (ObservationWindowDomain, error) {
	endTime := declaredEndTime
	if endTime.IsZero() || endTime.After(now) {
		endTime = now
	}

	if declaredStartTime.IsZero() {
		return ObservationWindowDomain{}, fmt.Errorf(
			"%w: 必須指定要看哪一段的起點", ErrObservationWindowValidation)
	}

	if !declaredStartTime.Before(endTime) {
		return ObservationWindowDomain{}, fmt.Errorf(
			"%w: 要看的那一段起點必須早於終點", ErrObservationWindowValidation)
	}

	return ObservationWindowDomain{
		startTime: declaredStartTime.UTC(),
		endTime:   endTime.UTC(),
	}, nil
}

func (observationWindowDomain ObservationWindowDomain) StartTime() time.Time {
	return observationWindowDomain.startTime
}

// EndTime is also the moment anything reading the window computes up to.
func (observationWindowDomain ObservationWindowDomain) EndTime() time.Time {
	return observationWindowDomain.endTime
}

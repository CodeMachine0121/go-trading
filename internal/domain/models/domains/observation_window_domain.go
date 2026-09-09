package domains

import (
	"fmt"
	"time"
)

// ObservationWindowDomain is the stretch of market a caller wants values for: where
// it begins and where it ends, both already settled.
//
// Settling them here is the whole point. "Up to when" has two readings that mean now
// — none named, and one that has not arrived — and a window that carried the raw
// declaration would leave every reader to apply that rule again, with nothing to stop
// two of them applying it differently. An instance existing means the two moments are
// real, in order, and no later than the present.
//
// It knows nothing about candles, coarseness or markets. How many slots the window
// holds is a question for the market's hours and the coarseness asked for; this only
// says which stretch is being asked about — which is why a backtest can hold one too.
type ObservationWindowDomain struct {
	startTime time.Time
	endTime   time.Time
}

// NewObservationWindowDomain settles the declared moments against now.
//
// A missing end means now, and so does one that has not arrived — the market cannot
// be read past the present, and refusing would break the ordinary case of a chart
// scrolled a little past its right edge.
//
// A missing start is refused rather than filled in. There is no length that could be
// meant by leaving it out, and picking one would answer about a stretch the caller
// never named and cannot see in the answer.
//
// A start that is not strictly earlier than the end is refused too, and the boundary
// belongs on this side: a stretch of no length is not a small question but an empty
// one, and letting it through would leave every market answering "nothing traded in
// this window" — true, and no help at all to whoever asked for nothing.
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

// StartTime is where the stretch begins.
func (observationWindowDomain ObservationWindowDomain) StartTime() time.Time {
	return observationWindowDomain.startTime
}

// EndTime is where the stretch ends, and therefore also the moment anything reading
// it computes up to. The two were once said separately; they are one moment, and
// saying it twice only gave them a chance to disagree.
func (observationWindowDomain ObservationWindowDomain) EndTime() time.Time {
	return observationWindowDomain.endTime
}

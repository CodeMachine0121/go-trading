package domains

import (
	"sort"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ReplayPaceDomain is the finest interval among a trading strategy's sources; coarser sources
// speak through it only with bars already finished, so a replay never reads the future.
type ReplayPaceDomain struct {
	interval         AggregationIntervalDomain
	coarsestInterval AggregationIntervalDomain
	warmupBarCount   int
}

// NewReplayPaceDomain expects at least one source; each lookback is counted in its own source's interval.
func NewReplayPaceDomain(
	sourceIntervals []AggregationIntervalDomain, sourceLookbackCounts []int,
) ReplayPaceDomain {
	interval := sourceIntervals[0]
	coarsestInterval := sourceIntervals[0]
	for _, sourceInterval := range sourceIntervals {
		if sourceInterval.duration < interval.duration {
			interval = sourceInterval
		}
		if sourceInterval.duration > coarsestInterval.duration {
			coarsestInterval = sourceInterval
		}
	}

	warmupBarCount := 0
	for sourceIndex, sourceInterval := range sourceIntervals {
		coveredDuration := time.Duration(sourceLookbackCounts[sourceIndex]) * sourceInterval.duration
		paceBarCount := int((coveredDuration + interval.duration - 1) / interval.duration)
		warmupBarCount = max(warmupBarCount, paceBarCount)
	}

	return ReplayPaceDomain{
		interval:         interval,
		coarsestInterval: coarsestInterval,
		warmupBarCount:   warmupBarCount,
	}
}

func (replayPaceDomain ReplayPaceDomain) Interval() AggregationIntervalDomain {
	return replayPaceDomain.interval
}

func (replayPaceDomain ReplayPaceDomain) CoarsestInterval() AggregationIntervalDomain {
	return replayPaceDomain.coarsestInterval
}

func (replayPaceDomain ReplayPaceDomain) WarmupBarCount() int {
	return replayPaceDomain.warmupBarCount
}

func (replayPaceDomain ReplayPaceDomain) MixesIntervals() bool {
	return replayPaceDomain.coarsestInterval.duration != replayPaceDomain.interval.duration
}

// SignalsAtPace gives each pace bar the signal of the last source bar closed by the pace bar's
// close, holding while none has closed yet.
func (replayPaceDomain ReplayPaceDomain) SignalsAtPace(
	sourceInterval AggregationIntervalDomain,
	sourceBarOpenTimes []time.Time,
	sourceSignals []SignalDomain,
	paceBarOpenTimes []time.Time,
) []SignalDomain {
	signalsAtPace := make([]SignalDomain, 0, len(paceBarOpenTimes))
	for _, paceBarOpenTime := range paceBarOpenTimes {
		paceBarCloseTime := paceBarOpenTime.Add(replayPaceDomain.interval.duration)
		closedSourceBarCount := sort.Search(len(sourceBarOpenTimes), func(sourceIndex int) bool {
			return sourceBarOpenTimes[sourceIndex].Add(sourceInterval.duration).After(paceBarCloseTime)
		})

		if closedSourceBarCount == 0 {
			signalsAtPace = append(signalsAtPace, NewSignalDomainOf(vo.SignalHold))

			continue
		}

		signalsAtPace = append(signalsAtPace, sourceSignals[closedSourceBarCount-1])
	}

	return signalsAtPace
}

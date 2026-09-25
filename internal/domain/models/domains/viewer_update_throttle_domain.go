package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// defaultUpdateIntervalCeiling applies when no usable interval was requested.
const defaultUpdateIntervalCeiling = 10 * time.Second

// ViewerUpdateThrottleDomain throttles per trading symbol, not per channel, and takes the
// current time as input so one round uses a single now.
type ViewerUpdateThrottleDomain struct {
	updateIntervalCeiling time.Duration
	lastAdmittedAt        time.Time
}

// NewViewerUpdateThrottleDomain seeds last-sent with startedAt so a new follow is not
// instantly due.
func NewViewerUpdateThrottleDomain(
	updateIntervalCeiling time.Duration, startedAt time.Time,
) *ViewerUpdateThrottleDomain {
	settledCeiling := defaultUpdateIntervalCeiling
	if updateIntervalCeiling > 0 {
		settledCeiling = updateIntervalCeiling
	}

	return &ViewerUpdateThrottleDomain{
		updateIntervalCeiling: settledCeiling,
		lastAdmittedAt:        startedAt.UTC(),
	}
}

// Admit always passes closed candles and passes forming candles at most once per ceiling,
// recording the decision.
func (viewerUpdateThrottleDomain *ViewerUpdateThrottleDomain) Admit(
	liveKCandle vo.LiveKCandleVo, now time.Time,
) bool {
	if !liveKCandle.Closed &&
		now.Sub(viewerUpdateThrottleDomain.lastAdmittedAt) < viewerUpdateThrottleDomain.updateIntervalCeiling {
		return false
	}

	viewerUpdateThrottleDomain.lastAdmittedAt = now.UTC()

	return true
}

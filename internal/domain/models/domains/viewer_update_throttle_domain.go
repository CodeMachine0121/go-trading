package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// defaultUpdateIntervalCeiling is how often a viewer's screen may be updated when
// nothing usable was asked for.
const defaultUpdateIntervalCeiling = 10 * time.Second

// ViewerUpdateThrottleDomain owns how often the picture of one trading symbol may be
// redrawn.
//
// It is per trading symbol rather than per channel because that is what a viewer is
// looking at: two symbols sharing a line are two pictures, and holding one back
// because the other just moved would make a busy neighbour into a slow chart.
//
// It reads no clock of its own: the moment is handed in, so one round of a follow
// judges everything against the same "now".
type ViewerUpdateThrottleDomain struct {
	updateIntervalCeiling time.Duration
	lastAdmittedAt        time.Time
}

// NewViewerUpdateThrottleDomain settles the rule up front, falling back to the
// stated default for anything not usable.
//
// startedAt seeds "last sent", so a follow that has only just begun is not instantly
// due for an update.
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

// Admit answers whether this reported candle should reach the viewers now, and
// records the answer.
//
// A candle that has closed is always admitted: it is that candle's last word, and a
// ceiling that swallowed it would lose it for good. A candle still forming is
// admitted only once per ceiling — the market moves many times a second, and
// forwarding every move makes the screen busy without making it clearer.
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

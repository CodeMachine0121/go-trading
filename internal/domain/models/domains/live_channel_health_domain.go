package domains

import "time"

// Defaults used whenever a caller hands over an unusable setting.
const (
	defaultQuietTimeout       = 30 * time.Second
	minimumQuietCheckInterval = time.Millisecond
	defaultMaximumRetryDelay  = 30 * time.Second
	initialRetryDelay         = time.Second
)

// LiveChannelHealthDomain judges one live channel's silence timeout and reconnect backoff per channel, since all symbols on a channel fail together; every call takes "now" explicitly.
type LiveChannelHealthDomain struct {
	quietTimeout      time.Duration
	maximumRetryDelay time.Duration
	lastReceivedAt    time.Time
	retryDelay        time.Duration
}

// NewLiveChannelHealthDomain falls back to defaults for unusable settings and seeds "last received" with startedAt so a new channel is not instantly quiet.
func NewLiveChannelHealthDomain(
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
	startedAt time.Time,
) *LiveChannelHealthDomain {
	settledQuietTimeout := defaultQuietTimeout
	if quietTimeout > 0 {
		settledQuietTimeout = quietTimeout
	}

	settledMaximumRetryDelay := defaultMaximumRetryDelay
	if maximumRetryDelay > 0 {
		settledMaximumRetryDelay = maximumRetryDelay
	}

	return &LiveChannelHealthDomain{
		quietTimeout:      settledQuietTimeout,
		maximumRetryDelay: settledMaximumRetryDelay,
		lastReceivedAt:    startedAt.UTC(),
		// The ceiling also binds the first gap.
		retryDelay: min(initialRetryDelay, settledMaximumRetryDelay),
	}
}

// MarkConnected restarts the silence clock but deliberately not the backoff, so a source that accepts and immediately drops connections is not hammered.
func (liveChannelHealthDomain *LiveChannelHealthDomain) MarkConnected(now time.Time) {
	liveChannelHealthDomain.lastReceivedAt = now.UTC()
}

// MarkReceived restarts the silence clock and resets the backoff, since only delivered data proves recovery.
func (liveChannelHealthDomain *LiveChannelHealthDomain) MarkReceived(now time.Time) {
	liveChannelHealthDomain.lastReceivedAt = now.UTC()
	liveChannelHealthDomain.retryDelay = min(
		initialRetryDelay, liveChannelHealthDomain.maximumRetryDelay)
}

// HasGoneQuiet treats silence as a dead connection, erring towards a needless reconnect over a frozen picture.
func (liveChannelHealthDomain *LiveChannelHealthDomain) HasGoneQuiet(now time.Time) bool {
	return now.UTC().Sub(liveChannelHealthDomain.lastReceivedAt) >= liveChannelHealthDomain.quietTimeout
}

// QuietCheckInterval is half the settled threshold, so a dead channel is noticed within one and a half thresholds.
func (liveChannelHealthDomain *LiveChannelHealthDomain) QuietCheckInterval() time.Duration {
	// Floored because a zero ticker interval panics in a background goroutine and takes down the whole process.
	return max(liveChannelHealthDomain.quietTimeout/2, minimumQuietCheckInterval)
}

// NextRetryDelay returns the current delay and doubles it up to the ceiling; it never gives up.
func (liveChannelHealthDomain *LiveChannelHealthDomain) NextRetryDelay() time.Duration {
	delay := liveChannelHealthDomain.retryDelay

	liveChannelHealthDomain.retryDelay = min(
		delay*2, liveChannelHealthDomain.maximumRetryDelay)

	return delay
}

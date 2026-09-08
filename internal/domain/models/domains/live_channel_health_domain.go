package domains

import "time"

// Defaults for the two rules that govern how one live channel behaves over time.
// They are the values the requirements name; a caller that hands over something
// unusable gets these rather than a broken channel.
const (
	defaultQuietTimeout      = 30 * time.Second
	defaultMaximumRetryDelay = 30 * time.Second
	initialRetryDelay        = time.Second
)

// LiveChannelHealthDomain owns how one live channel is judged over time: how long a
// silence means the feed died, and how far apart the attempts to reopen it grow.
//
// It is per channel rather than per trading symbol because that is what fails: the
// symbols travelling on one channel go quiet together, come back together, and are
// waited for together. Judging them one by one would multiply one outage by however
// many symbols happened to be on the line.
//
// It reads no clock of its own: every question is asked against a moment handed in,
// so one round judges everything against the same "now".
type LiveChannelHealthDomain struct {
	quietTimeout      time.Duration
	maximumRetryDelay time.Duration
	lastReceivedAt    time.Time
	retryDelay        time.Duration
}

// NewLiveChannelHealthDomain settles both rules up front, falling back to the stated
// defaults for anything not usable. An instance existing therefore means the rules
// are usable — the caller never has to check them again.
//
// startedAt seeds "last received", so a channel that has only just begun is not
// instantly considered quiet.
func NewLiveChannelHealthDomain(
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
	startedAt time.Time,
) *LiveChannelHealthDomain {
	return &LiveChannelHealthDomain{
		quietTimeout:      positiveOr(quietTimeout, defaultQuietTimeout),
		maximumRetryDelay: positiveOr(maximumRetryDelay, defaultMaximumRetryDelay),
		lastReceivedAt:    startedAt.UTC(),
		// The ceiling binds every gap, the first one included. A caller who asked for
		// gaps no longer than half a second did not mean "except the first".
		retryDelay: min(initialRetryDelay, positiveOr(maximumRetryDelay, defaultMaximumRetryDelay)),
	}
}

// MarkConnected records that a channel was opened, which is when the silence starts
// being measured from. A channel that has only just opened has not been quiet for
// however long the previous one was.
//
// It deliberately does not touch the retry gap. Opening a connection proves nothing
// about a source: one that accepts every connection and immediately drops it would
// otherwise reset the gap on every attempt and be hammered once a second forever,
// which is the exact failure the growing gap exists to prevent. Only data arriving
// counts as recovery.
func (liveChannelHealthDomain *LiveChannelHealthDomain) MarkConnected(now time.Time) {
	liveChannelHealthDomain.lastReceivedAt = now.UTC()
}

// MarkReceived records that the channel delivered something, which is the only thing
// a source can do that proves it works: the silence starts again from here, and the
// gap earned while it was broken is given back.
func (liveChannelHealthDomain *LiveChannelHealthDomain) MarkReceived(now time.Time) {
	liveChannelHealthDomain.lastReceivedAt = now.UTC()
	liveChannelHealthDomain.retryDelay = min(
		initialRetryDelay, liveChannelHealthDomain.maximumRetryDelay)
}

// HasGoneQuiet answers whether the channel has died without saying so. A connection
// that looks open but has stopped delivering is how this kind of channel usually
// fails, so silence is treated as death rather than as calm.
//
// A market that genuinely traded nothing looks identical from here. Getting it wrong
// costs one needless reconnection; not getting it costs a viewer staring at a frozen
// picture, so this errs towards calling it dead.
func (liveChannelHealthDomain *LiveChannelHealthDomain) HasGoneQuiet(now time.Time) bool {
	return now.UTC().Sub(liveChannelHealthDomain.lastReceivedAt) >= liveChannelHealthDomain.quietTimeout
}

// QuietCheckInterval is how often the silence is worth measuring: half the
// threshold, so that a channel which died is noticed within one and a half
// thresholds rather than two.
//
// It is answered here rather than by the caller because it is derived from the
// threshold after it has been settled — a caller working it out from the raw setting
// would have to know the fallback as well, and would then hold a second copy of it.
func (liveChannelHealthDomain *LiveChannelHealthDomain) QuietCheckInterval() time.Duration {
	return liveChannelHealthDomain.quietTimeout / 2
}

// NextRetryDelay hands out how long to wait before trying again, and doubles it for
// the time after that up to the ceiling. Growing the gap keeps a source that is
// briefly unwell from being hammered, and the ceiling keeps a source that recovers
// after an hour from being ignored for another one. It never gives up.
func (liveChannelHealthDomain *LiveChannelHealthDomain) NextRetryDelay() time.Duration {
	delay := liveChannelHealthDomain.retryDelay

	liveChannelHealthDomain.retryDelay = min(
		delay*2, liveChannelHealthDomain.maximumRetryDelay)

	return delay
}

// positiveOr keeps a duration that makes sense as a rule and replaces one that does
// not. Zero or less would mean "no rule at all", which is never what a caller
// leaving a setting unfilled meant.
func positiveOr(duration time.Duration, fallback time.Duration) time.Duration {
	if duration <= 0 {
		return fallback
	}

	return duration
}

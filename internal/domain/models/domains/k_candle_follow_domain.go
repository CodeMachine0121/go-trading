package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// Defaults for the three rules that govern how one follow behaves over time. They
// are the values the requirements name; a caller that hands over something unusable
// gets these rather than a broken follow.
const (
	defaultUpdateIntervalCeiling = 10 * time.Second
	defaultQuietTimeout          = 30 * time.Second
	defaultMaximumRetryDelay     = 30 * time.Second
	initialRetryDelay            = time.Second
)

// KCandleFollowDomain owns every rule of one live follow that carries a number:
// how often a viewer's screen may be updated, how long a silence means the feed
// died, and how far apart retries grow.
//
// They live on one object because they share one reason to change — how a follow
// should behave as time passes — and because keeping them here is what lets them be
// pinned by a table test instead of by disconnecting a real socket. Nothing in this
// file knows what a connection is.
//
// It reads no clock of its own: every question is asked against a moment handed in,
// so one round of the follow judges everything against the same "now".
type KCandleFollowDomain struct {
	updateIntervalCeiling time.Duration
	quietTimeout          time.Duration
	maximumRetryDelay     time.Duration
	lastAdmittedAt        time.Time
	lastReceivedAt        time.Time
	retryDelay            time.Duration
}

// NewKCandleFollowDomain settles the three rules up front, falling back to the
// stated defaults for anything not usable. An instance existing therefore means the
// rules are usable — the caller never has to check them again.
//
// startedAt seeds both "last sent" and "last received", so a follow that has only
// just begun is neither instantly due for an update nor instantly considered quiet.
func NewKCandleFollowDomain(
	updateIntervalCeiling time.Duration,
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
	startedAt time.Time,
) *KCandleFollowDomain {
	return &KCandleFollowDomain{
		updateIntervalCeiling: positiveOr(updateIntervalCeiling, defaultUpdateIntervalCeiling),
		quietTimeout:          positiveOr(quietTimeout, defaultQuietTimeout),
		maximumRetryDelay:     positiveOr(maximumRetryDelay, defaultMaximumRetryDelay),
		lastAdmittedAt:        startedAt.UTC(),
		lastReceivedAt:        startedAt.UTC(),
		// The ceiling binds every gap, the first one included. A caller who asked for
		// gaps no longer than half a second did not mean "except the first".
		retryDelay: min(initialRetryDelay, positiveOr(maximumRetryDelay, defaultMaximumRetryDelay)),
	}
}

// Admit answers whether this reported candle should reach the viewers now, and
// records the answer.
//
// A candle that has closed is always admitted: it is that candle's last word, and a
// ceiling that swallowed it would lose it for good. A candle still forming is
// admitted only once per ceiling — the market moves many times a second, and
// forwarding every move makes the screen busy without making it clearer.
//
// Receiving anything at all counts as the feed being alive, whether or not it is
// passed on.
func (kCandleFollowDomain *KCandleFollowDomain) Admit(
	liveKCandle vo.LiveKCandleVo, now time.Time,
) bool {
	kCandleFollowDomain.lastReceivedAt = now.UTC()

	if !liveKCandle.Closed && now.Sub(kCandleFollowDomain.lastAdmittedAt) < kCandleFollowDomain.updateIntervalCeiling {
		return false
	}

	kCandleFollowDomain.lastAdmittedAt = now.UTC()

	return true
}

// HasGoneQuiet answers whether the feed has died without saying so. A connection
// that looks open but has stopped delivering is how this kind of channel usually
// fails, so silence is treated as death rather than as calm.
//
// A market that genuinely traded nothing looks identical from here. Getting it
// wrong costs one needless reconnection; not getting it costs a viewer staring at a
// frozen picture, so this errs towards calling it dead.
func (kCandleFollowDomain *KCandleFollowDomain) HasGoneQuiet(now time.Time) bool {
	return now.UTC().Sub(kCandleFollowDomain.lastReceivedAt) >= kCandleFollowDomain.quietTimeout
}

// QuietCheckInterval is how often the silence is worth measuring: half the
// threshold, so that a feed which died is noticed within one and a half thresholds
// rather than two.
//
// It is answered here rather than by the caller because it is derived from the
// threshold after it has been settled — a caller working it out from the raw
// setting would have to know the fallback as well, and would then hold a second
// copy of it.
func (kCandleFollowDomain *KCandleFollowDomain) QuietCheckInterval() time.Duration {
	return kCandleFollowDomain.quietTimeout / 2
}

// NextRetryDelay hands out how long to wait before trying again, and doubles it for
// the time after that up to the ceiling. Growing the gap keeps a source that is
// briefly unwell from being hammered, and the ceiling keeps a source that recovers
// after an hour from being ignored for another one. It never gives up.
func (kCandleFollowDomain *KCandleFollowDomain) NextRetryDelay() time.Duration {
	delay := kCandleFollowDomain.retryDelay

	kCandleFollowDomain.retryDelay = min(delay*2, kCandleFollowDomain.maximumRetryDelay)

	return delay
}

// MarkFollowing records that the source is answering again: the retry gap goes back
// to its shortest, and the feed counts as alive as of now. Without this a follow
// that recovers would keep the long gap it earned while it was broken.
func (kCandleFollowDomain *KCandleFollowDomain) MarkFollowing(now time.Time) {
	kCandleFollowDomain.retryDelay = min(initialRetryDelay, kCandleFollowDomain.maximumRetryDelay)
	kCandleFollowDomain.lastReceivedAt = now.UTC()
}

// positiveOr keeps a duration that makes sense as a rule and replaces one that does
// not. Zero or less would mean "no ceiling at all", which is never what a caller
// leaving a setting unfilled meant.
func positiveOr(duration time.Duration, fallback time.Duration) time.Duration {
	if duration <= 0 {
		return fallback
	}

	return duration
}

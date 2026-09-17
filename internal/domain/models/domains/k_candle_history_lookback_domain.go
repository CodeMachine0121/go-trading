package domains

import (
	"errors"
	"fmt"
	"time"
)

// ErrKCandleHistoryLookback marks a lookback that is not a stretch of history
// anybody could fetch: none at all, backwards, or further than the system is
// willing to reach in one go.
var ErrKCandleHistoryLookback = errors.New("k candle history lookback rejected")

// KCandleHistoryLookbackDomain is how far back one history sync reaches, in whole
// days.
//
// It is a domain model rather than a number because it is a number with conditions
// attached, and because the refusal has to say something useful: a caller turned
// away has to learn what to ask for instead, not be left halving the figure until
// something works.
//
// The ceiling comes in from outside. How far this system is willing to reach in one
// request is an operator's decision about what it will do to the market source and
// to its own memory — not something the domain knows.
type KCandleHistoryLookbackDomain struct {
	days int
}

// NewKCandleHistoryLookbackDomain judges a lookback and hands back one that holds.
//
// **There is no default.** Saying how far back is the whole point of this request:
// filling in a figure for somebody who left it out would answer a question they did
// not ask, with the one number they were trying to choose.
func NewKCandleHistoryLookbackDomain(
	days int, ceilingDays int,
) (KCandleHistoryLookbackDomain, error) {
	if days < 1 {
		return KCandleHistoryLookbackDomain{}, fmt.Errorf(
			"%w: 回溯天數必須至少 1 天，收到 %d", ErrKCandleHistoryLookback, days)
	}

	if days > ceilingDays {
		return KCandleHistoryLookbackDomain{}, fmt.Errorf(
			"%w: 回溯天數最多 %d 天，收到 %d", ErrKCandleHistoryLookback, ceilingDays, days)
	}

	return KCandleHistoryLookbackDomain{days: days}, nil
}

// Duration is the stretch as a length of time, which is what a window is built from.
func (kCandleHistoryLookbackDomain KCandleHistoryLookbackDomain) Duration() time.Duration {
	return time.Duration(kCandleHistoryLookbackDomain.days) * 24 * time.Hour
}

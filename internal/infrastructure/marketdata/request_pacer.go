package marketdata

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// requestPacer holds a source to a rate it has agreed to answer at, making every
// caller wait its turn.
//
// It exists because nothing above here knows how many requests one call becomes.
// Asking for a stretch of candles is one call to a proxy and anywhere between one
// and several thousand requests to the venue, and the venue's allowance is counted
// in requests. A source pushed past it answers 429 and then, on some venues, stops
// answering this address at all for a while — so pacing is not politeness, it is the
// difference between a long fetch finishing and a long fetch getting the system
// banned partway through.
//
// It paces one request at a time rather than letting a burst through. A burst is
// only ever worth having when the work is short, and the runs that need pacing are
// exactly the long ones; allowing one would spend the whole allowance in the first
// second of a fetch that then has to wait out the rest of the minute anyway.
//
// It lives beside the proxies rather than in the domain because a venue's allowance
// is a fact about that venue, in the same way its address and its wire format are.
type requestPacer struct {
	limiter *rate.Limiter
}

// newRequestPacer paces to the given allowance. A rate of nothing paces nothing,
// which is what the tests that are not about pacing run on.
func newRequestPacer(requestsPerMinute int) requestPacer {
	if requestsPerMinute <= 0 {
		return requestPacer{limiter: rate.NewLimiter(rate.Inf, 1)}
	}

	return requestPacer{
		limiter: rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), 1),
	}
}

// waitForTurn blocks until this request may be sent, or until the caller gives up.
// A caller that gave up is told so rather than being let through.
func (pacer requestPacer) waitForTurn(executionContext context.Context) error {
	return pacer.limiter.Wait(executionContext)
}

package marketdata

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// RequestPacer spaces requests to a venue's per-minute allowance with no burst, since exceeding it can get the address banned; build one per venue in the composition root and share it across that venue's proxies.
type RequestPacer struct {
	limiter *rate.Limiter
}

// NewRequestPacer with a zero rate does not pace at all.
func NewRequestPacer(requestsPerMinute int) RequestPacer {
	if requestsPerMinute <= 0 {
		return RequestPacer{limiter: rate.NewLimiter(rate.Inf, 1)}
	}

	return RequestPacer{
		limiter: rate.NewLimiter(rate.Every(time.Minute/time.Duration(requestsPerMinute)), 1),
	}
}

// Limiter is exposed so tests can verify two venues do not share one pacer, which comparing rates cannot detect.
func (requestPacer RequestPacer) Limiter() *rate.Limiter {
	return requestPacer.limiter
}

// WaitForTurn blocks until the request may be sent, returning an error if the context is done first.
func (pacer RequestPacer) WaitForTurn(executionContext context.Context) error {
	return pacer.limiter.Wait(executionContext)
}

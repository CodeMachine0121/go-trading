package domains

import (
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"golang.org/x/time/rate"
)

// minimumSweepInterval keeps a fast-refilling budget from sweeping on nearly every request.
const minimumSweepInterval = time.Minute

// RequestAllowanceLedgerDomain keeps one refilling allowance per requester and forgets those that have refilled,
// which is lossless because a full allowance behaves exactly like a fresh one.
type RequestAllowanceLedgerDomain struct {
	mutex         sync.Mutex
	refillRate    rate.Limit
	burst         int
	sweepInterval time.Duration
	lastSweptAt   time.Time
	allowances    map[string]*rate.Limiter
}

// NewRequestAllowanceLedgerDomain raises a zero or negative budget to one per minute, saving up one.
func NewRequestAllowanceLedgerDomain(budget vo.RequestBudgetVo) *RequestAllowanceLedgerDomain {
	requestsPerMinute := max(1, budget.RequestsPerMinute)
	burst := max(1, budget.Burst)

	return &RequestAllowanceLedgerDomain{
		refillRate:    rate.Every(time.Minute / time.Duration(requestsPerMinute)),
		burst:         burst,
		sweepInterval: max(minimumSweepInterval, time.Duration(burst)*time.Minute/time.Duration(requestsPerMinute)),
		allowances:    make(map[string]*rate.Limiter),
	}
}

// Admit spends one allowance, or refuses with the wait until the next one without spending anything.
func (ledger *RequestAllowanceLedgerDomain) Admit(requesterKey string, now time.Time) error {
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()

	if now.Sub(ledger.lastSweptAt) >= ledger.sweepInterval {
		for key, allowance := range ledger.allowances {
			if allowance.TokensAt(now) >= float64(ledger.burst) {
				delete(ledger.allowances, key)
			}
		}
		ledger.lastSweptAt = now
	}

	allowance, isKnown := ledger.allowances[requesterKey]
	if !isKnown {
		allowance = rate.NewLimiter(ledger.refillRate, ledger.burst)
		ledger.allowances[requesterKey] = allowance
	}

	reservation := allowance.ReserveN(now, 1)
	if delay := reservation.DelayFrom(now); delay > 0 {
		reservation.CancelAt(now)

		return RequestRateExceededError{RetryAfter: delay}
	}

	return nil
}

func (ledger *RequestAllowanceLedgerDomain) TrackedRequesterCount() int {
	ledger.mutex.Lock()
	defer ledger.mutex.Unlock()

	return len(ledger.allowances)
}

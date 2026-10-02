package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
)

// leaseAcquiredAt and the moments below are written out so assertions state the requirement, not the arithmetic.
var leaseAcquiredAt = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)

func TestJobLeadershipTermDomain(t *testing.T) {
	testCases := []struct {
		name              string
		leaseDuration     time.Duration
		safetyMargin      time.Duration
		expectedExpiresAt time.Time
		expectedHeldUntil time.Time
	}{
		{
			name:              "the stored lease lasts thirty seconds and the replica lets go five seconds early",
			leaseDuration:     30 * time.Second,
			safetyMargin:      5 * time.Second,
			expectedExpiresAt: time.Date(2026, 10, 2, 8, 0, 30, 0, time.UTC),
			expectedHeldUntil: time.Date(2026, 10, 2, 8, 0, 25, 0, time.UTC),
		},
		{
			name:              "a margin as long as the lease is cut to half so the replica is ever on duty",
			leaseDuration:     30 * time.Second,
			safetyMargin:      30 * time.Second,
			expectedExpiresAt: time.Date(2026, 10, 2, 8, 0, 30, 0, time.UTC),
			expectedHeldUntil: time.Date(2026, 10, 2, 8, 0, 15, 0, time.UTC),
		},
		{
			name:              "a negative margin counts as none",
			leaseDuration:     30 * time.Second,
			safetyMargin:      -5 * time.Second,
			expectedExpiresAt: time.Date(2026, 10, 2, 8, 0, 30, 0, time.UTC),
			expectedHeldUntil: time.Date(2026, 10, 2, 8, 0, 30, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			term := domains.NewJobLeadershipTermDomain(testCase.leaseDuration, testCase.safetyMargin)

			assert.Equal(t, testCase.expectedExpiresAt, term.ExpiresAt(leaseAcquiredAt))
			assert.Equal(t, testCase.expectedHeldUntil, term.HeldUntil(leaseAcquiredAt))
		})
	}
}

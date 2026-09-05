package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
)

var sessionNow = time.Date(2026, 9, 5, 8, 0, 0, 0, time.UTC)

func aSession(expiresAt time.Time, revokedAt *time.Time) entities.Session {
	return entities.Session{
		ID:                 11,
		UserID:             7,
		ChainID:            "a-chain",
		RefreshTokenDigest: "a-digest",
		ExpiresAt:          expiresAt,
		RevokedAt:          revokedAt,
	}
}

func TestSessionDomainTellsRevokedAndExpiredApart(t *testing.T) {
	revokedAt := sessionNow.Add(-time.Hour)

	testCases := []struct {
		name            string
		session         entities.Session
		expectedRevoked bool
		expectedExpired bool
	}{
		{
			name:    "still good",
			session: aSession(sessionNow.Add(24*time.Hour), nil),
		},
		{
			name:            "ended, but its moment has not passed",
			session:         aSession(sessionNow.Add(24*time.Hour), &revokedAt),
			expectedRevoked: true,
		},
		{
			name:            "its moment passed a second ago",
			session:         aSession(sessionNow.Add(-time.Second), nil),
			expectedExpired: true,
		},
		{
			name: "its moment is exactly now",
			// An expiry is the first instant something stops working, not the last
			// instant it still does.
			session:         aSession(sessionNow, nil),
			expectedExpired: true,
		},
		{
			name:    "one second of life left",
			session: aSession(sessionNow.Add(time.Second), nil),
		},
		{
			name: "both ended and expired",
			// The two are asked separately because they lead to different actions:
			// expired is simply refused, ended-but-still-presented means the proof
			// was copied and the whole chain has to go.
			session:         aSession(sessionNow.Add(-time.Second), &revokedAt),
			expectedRevoked: true,
			expectedExpired: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sessionDomain := domains.NewSessionDomain(testCase.session)

			assert.Equal(t, testCase.expectedRevoked, sessionDomain.Revoked())
			assert.Equal(t, testCase.expectedExpired, sessionDomain.Expired(sessionNow))
		})
	}
}

func TestSessionDomainHandsOutWhatRotatingNeedsToKnow(t *testing.T) {
	sessionDomain := domains.NewSessionDomain(aSession(sessionNow.Add(24*time.Hour), nil))

	assert.Equal(t, uint(11), sessionDomain.ID())
	assert.Equal(t, uint(7), sessionDomain.UserID())
	assert.Equal(t, "a-chain", sessionDomain.ChainID())
}

func TestSessionDomainRenewedStaysOnTheSameChain(t *testing.T) {
	sessionDomain := domains.NewSessionDomain(aSession(sessionNow.Add(24*time.Hour), nil))

	next := sessionDomain.Renewed("a-newer-digest", sessionNow, 30*24*time.Hour)

	assert.Equal(t, "a-chain", next.ChainID,
		"換了鏈，登出與盜用偵測就撤不到這一段——它們撤的都是整條鏈")
	assert.Equal(t, uint(7), next.UserID)
	assert.Equal(t, "a-newer-digest", next.RefreshTokenDigest)
	assert.Nil(t, next.RevokedAt)
	assert.Equal(t, uint(0), next.ID, "還沒被存下來的那一段不帶自己的識別碼")
}

func TestSessionDomainRenewedStartsTheClockAgainFromNow(t *testing.T) {
	// Carrying the old expiry forward would make a session that can never be kept
	// alive past its original month, however often it is used. Counting from now is
	// what makes "keep using it and you stay in; leave it and you do not" true.
	testCases := []struct {
		name              string
		now               time.Time
		lifetime          time.Duration
		expectedExpiresAt time.Time
	}{
		{
			name:              "renewed the moment it was issued",
			now:               sessionNow,
			lifetime:          30 * 24 * time.Hour,
			expectedExpiresAt: time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC),
		},
		{
			name:              "renewed five days later",
			now:               time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC),
			lifetime:          30 * 24 * time.Hour,
			expectedExpiresAt: time.Date(2026, 10, 10, 8, 0, 0, 0, time.UTC),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sessionDomain := domains.NewSessionDomain(aSession(sessionNow.Add(24*time.Hour), nil))

			next := sessionDomain.Renewed("a-newer-digest", testCase.now, testCase.lifetime)

			assert.Equal(t, testCase.expectedExpiresAt, next.ExpiresAt)
		})
	}
}

package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// SessionDomain keeps "revoked" and "expired" separate because presenting a revoked session means a copied renewal proof and the whole chain must be torn down.
type SessionDomain struct {
	session entities.Session
}

func NewSessionDomain(session entities.Session) SessionDomain {
	return SessionDomain{session: session}
}

// Revoked covers renewal, sign-out and chain teardown.
func (sessionDomain SessionDomain) Revoked() bool {
	return sessionDomain.session.RevokedAt != nil
}

// Expired treats the expiry instant itself as past.
func (sessionDomain SessionDomain) Expired(now time.Time) bool {
	return !now.Before(sessionDomain.session.ExpiresAt)
}

func (sessionDomain SessionDomain) ID() uint {
	return sessionDomain.session.ID
}

func (sessionDomain SessionDomain) UserID() uint {
	return sessionDomain.session.UserID
}

// ChainID identifies the sign-in; ending a chain is what both sign-out and reuse detection come down to.
func (sessionDomain SessionDomain) ChainID() string {
	return sessionDomain.session.ChainID
}

// Renewed restarts the expiry from now rather than carrying it forward, so an active session never needs re-sign-in but an idle one does.
func (sessionDomain SessionDomain) Renewed(
	refreshTokenDigest string, now time.Time, lifetime time.Duration,
) entities.Session {
	return entities.Session{
		UserID:             sessionDomain.session.UserID,
		ChainID:            sessionDomain.session.ChainID,
		RefreshTokenDigest: refreshTokenDigest,
		ExpiresAt:          now.Add(lifetime).UTC(),
	}
}

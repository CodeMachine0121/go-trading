package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// SessionDomain holds one session and answers whether it still counts.
//
// It answers "has it been revoked" and "has it expired" as two separate questions
// rather than one "is it usable", and that separation is the most important thing in
// this model. The two lead to different actions: an expired session is simply
// refused, while a revoked one that somebody is still presenting means the proof was
// used twice — which can only be a copy — and the whole chain has to go. Collapsed
// into one boolean, that distinction disappears, and with it the only defence this
// system has against a stolen renewal proof.
type SessionDomain struct {
	session entities.Session
}

func NewSessionDomain(session entities.Session) SessionDomain {
	return SessionDomain{session: session}
}

// Revoked says whether this session has already been ended — by being renewed away
// from, by a sign-out, or by a chain that was torn down.
func (sessionDomain SessionDomain) Revoked() bool {
	return sessionDomain.session.RevokedAt != nil
}

// Expired says whether this session's moment has passed. The moment itself counts as
// past: an expiry is the first instant something stops working, not the last instant
// it still does.
func (sessionDomain SessionDomain) Expired(now time.Time) bool {
	return !now.Before(sessionDomain.session.ExpiresAt)
}

// Usable is the two of them together, for the caller that only needs the conclusion.
func (sessionDomain SessionDomain) Usable(now time.Time) bool {
	return !sessionDomain.Revoked() && !sessionDomain.Expired(now)
}

// ID is which stored session this is, for the rotation that has to end it.
func (sessionDomain SessionDomain) ID() uint {
	return sessionDomain.session.ID
}

// UserID is whose session this is.
func (sessionDomain SessionDomain) UserID() uint {
	return sessionDomain.session.UserID
}

// ChainID is the one sign-in this session belongs to. Ending a chain is what both
// signing out and detecting a reused proof come down to.
func (sessionDomain SessionDomain) ChainID() string {
	return sessionDomain.session.ChainID
}

// Renewed is the session this one becomes: same person, same chain, a new proof, and
// a fresh expiry counted from now.
//
// Counting from now rather than carrying the old expiry forward is what makes "keep
// using it and you never sign in again; leave it alone too long and you do" true. It
// is also why renewing does not extend a session indefinitely by accident — the
// clock restarts, it does not accumulate.
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

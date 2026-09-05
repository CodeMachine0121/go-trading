package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_session_repository.go -destination=mocks/mock_i_session_repository.go -package=mocks

// ISessionRepository stores the sign-ins this system is currently honouring.
//
// Every method here is one whole business action. That is deliberate: the one thing
// this interface must not do is hand back two calls that a caller has to put in the
// right order, because two of the four things it does are only correct when they
// happen together.
type ISessionRepository interface {
	// Save stores a newly opened session and returns it as stored.
	Save(executionContext context.Context, session entities.Session) (entities.Session, error)
	// FindOneByDigest returns the session a renewal proof belongs to, or
	// ErrSessionNotFound.
	//
	// It looks a session up by what was stored in place of the proof, which is only
	// possible because that derivation is the same every time. A password proof
	// could not be looked up this way, and does not need to be.
	FindOneByDigest(executionContext context.Context, refreshTokenDigest string) (entities.Session, error)
	// Rotate ends one session and opens its successor, both at once.
	//
	// Both at once is the whole method. Ending and opening as two calls leaves a
	// window where the old proof is dead and the new one was never written — and the
	// person holding them has two proofs, neither of which works, with nothing
	// anywhere to explain why.
	Rotate(
		executionContext context.Context, previousSessionID uint, next entities.Session,
	) (entities.Session, error)
	// RevokeChain ends every session of one sign-in, including the one still good.
	//
	// Signing out and catching a reused proof both come down to this, and they come
	// down to the same thing for the same reason: the unit being ended is the
	// sign-in, not the individual proof. Ending a chain that is already ended, or one
	// that was never there, is not a failure — the outcome asked for is already true.
	RevokeChain(executionContext context.Context, chainID string) error
}

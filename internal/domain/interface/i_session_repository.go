package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_session_repository.go -destination=mocks/mock_i_session_repository.go -package=mocks

// ISessionRepository exposes each business action as one method so operations that must happen together cannot be mis-sequenced.
type ISessionRepository interface {
	Save(executionContext context.Context, session entities.Session) (entities.Session, error)
	// FindOneByDigest returns ErrSessionNotFound when no session matches the refresh token digest.
	FindOneByDigest(executionContext context.Context, refreshTokenDigest string) (entities.Session, error)
	// Rotate atomically ends a session and opens its successor, only if it had not already ended; otherwise ErrSessionAlreadyRotated, which is what makes a refresh token single-use under races.
	Rotate(
		executionContext context.Context, previousSessionID uint, next entities.Session,
	) (entities.Session, error)
	// RevokeChain ends every session of one sign-in; revoking an ended or unknown chain is a no-op.
	RevokeChain(executionContext context.Context, chainID string) error
}

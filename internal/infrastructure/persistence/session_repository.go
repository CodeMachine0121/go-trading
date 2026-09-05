package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SessionRefreshTokenDigestIndex is the unique index on what a session stores in
// place of its renewal proof. It is repeated from the entity's tag because a struct
// tag cannot hold a constant, and it is exported so a test needing no database holds
// the two spellings together.
const SessionRefreshTokenDigestIndex = "idx_sessions_refresh_token_digest"

// SessionRepository stores the sign-ins this system is currently honouring.
type SessionRepository struct {
	database *gorm.DB
}

func NewSessionRepository(database *gorm.DB) *SessionRepository {
	return &SessionRepository{database: database}
}

// Save stores a newly opened session.
func (sessionRepository *SessionRepository) Save(
	executionContext context.Context, session entities.Session,
) (entities.Session, error) {
	result := sessionRepository.database.WithContext(executionContext).Create(&session)
	if result.Error != nil {
		return entities.Session{}, fmt.Errorf("save session: %w", result.Error)
	}

	return session, nil
}

// FindOneByDigest returns the session a renewal proof belongs to.
//
// It deliberately does not filter out revoked or expired sessions. A revoked one has
// to come back, because a revoked proof being presented is the single signal this
// system has that a proof was copied — filtering it out here would turn theft into
// an ordinary "not found" and lose the whole defence.
func (sessionRepository *SessionRepository) FindOneByDigest(
	executionContext context.Context, refreshTokenDigest string,
) (entities.Session, error) {
	session := entities.Session{}

	// Spelled out rather than given as a struct for the same reason as the user
	// lookup: GORM drops zero-valued struct fields, and an empty digest would
	// become no condition at all.
	result := sessionRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "refresh_token_digest", Value: refreshTokenDigest}).
		First(&session)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.Session{}, domains.ErrSessionNotFound
	}
	if result.Error != nil {
		return entities.Session{}, fmt.Errorf("find session: %w", result.Error)
	}

	return session, nil
}

// Rotate ends one session and opens its successor inside one transaction.
//
// One transaction is the entire reason this is a method rather than two. Apart, a
// failure between them leaves the old proof dead and the new one unwritten, and the
// person holding both has two proofs that do nothing and no way to find out why.
func (sessionRepository *SessionRepository) Rotate(
	executionContext context.Context, previousSessionID uint, next entities.Session,
) (entities.Session, error) {
	rotatedSession := entities.Session{}

	transactionError := sessionRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The revocation time comes from the database rather than from a clock
			// this code holds, because it has to sit on the same timeline as the
			// row's own timestamps — and because two clocks is one more than the
			// question "when was this ended" can have.
			//
			// Only a session that has not already ended may be rotated, and that
			// condition is on the write rather than checked beforehand. Checking
			// beforehand cannot work: two renewals carrying the same proof both read
			// a session that is still good, and both would then write. Here the
			// second one updates no rows, because the first one's row no longer
			// matches — which is how "a proof works once" becomes a fact the database
			// enforces instead of a fact two readers each believe separately.
			//
			// It is also what stops a rotation from quietly undoing a sign-out: a
			// chain that was revoked a moment ago has no row left for this to match.
			revoked := transaction.
				Model(&entities.Session{}).
				Where(clause.Eq{Column: "id", Value: previousSessionID}).
				Where(clause.Eq{Column: "revoked_at", Value: nil}).
				Update("revoked_at", gorm.Expr("now()"))
			if revoked.Error != nil {
				return fmt.Errorf("revoke previous session: %w", revoked.Error)
			}
			if revoked.RowsAffected == 0 {
				return domains.ErrSessionAlreadyRotated
			}

			if created := transaction.Create(&next); created.Error != nil {
				return fmt.Errorf("save renewed session: %w", created.Error)
			}

			rotatedSession = next

			return nil
		})

	if transactionError != nil {
		return entities.Session{}, transactionError
	}

	return rotatedSession, nil
}

// RevokeChain ends every session of one sign-in.
//
// Sessions already ended are left with the moment they were ended: it is the first
// time that answers "when did this stop", and overwriting it would erase exactly the
// trail somebody would follow to find out what happened.
//
// Ending a chain that is not there, or is already ended, is not a failure — what was
// asked for is already true.
func (sessionRepository *SessionRepository) RevokeChain(
	executionContext context.Context, chainID string,
) error {
	result := sessionRepository.database.WithContext(executionContext).
		Model(&entities.Session{}).
		Where(clause.Eq{Column: "chain_id", Value: chainID}).
		Where(clause.Eq{Column: "revoked_at", Value: nil}).
		Update("revoked_at", gorm.Expr("now()"))
	if result.Error != nil {
		return fmt.Errorf("revoke session chain: %w", result.Error)
	}

	return nil
}

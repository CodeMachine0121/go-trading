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

// SessionRefreshTokenDigestIndex duplicates the entity's struct tag, which cannot reference a constant.
const SessionRefreshTokenDigestIndex = "idx_sessions_refresh_token_digest"

type SessionRepository struct {
	database *gorm.DB
}

func NewSessionRepository(database *gorm.DB) *SessionRepository {
	return &SessionRepository{database: database}
}

func (sessionRepository *SessionRepository) Save(
	executionContext context.Context, session entities.Session,
) (entities.Session, error) {
	result := sessionRepository.database.WithContext(executionContext).Create(&session)
	if result.Error != nil {
		return entities.Session{}, fmt.Errorf("save session: %w", result.Error)
	}

	return session, nil
}

// FindOneByDigest does not filter revoked or expired sessions, because presenting a revoked token is the only signal of theft.
func (sessionRepository *SessionRepository) FindOneByDigest(
	executionContext context.Context, refreshTokenDigest string,
) (entities.Session, error) {
	session := entities.Session{}

	// A string condition, because GORM drops zero-valued struct fields and an empty digest would match everything.
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

// Rotate revokes one session and creates its successor in one transaction, so a failure cannot leave both tokens dead.
func (sessionRepository *SessionRepository) Rotate(
	executionContext context.Context, previousSessionID uint, next entities.Session,
) (entities.Session, error) {
	rotatedSession := entities.Session{}

	transactionError := sessionRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The revocation time comes from the database clock, and the not-yet-revoked condition sits on the update itself, so a concurrent renewal or sign-out makes this update zero rows.
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

// RevokeChain ends every session of one sign-in, keeping existing revocation times; revoking a missing or already-revoked chain is not an error.
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

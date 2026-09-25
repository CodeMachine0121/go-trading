package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserEmailIndex repeats the entity's index tag (tags cannot hold constants); it is exported so a database-free test asserts the two spellings agree.
const UserEmailIndex = "idx_users_email"

type UserRepository struct {
	database *gorm.DB
}

func NewUserRepository(database *gorm.DB) *UserRepository {
	return &UserRepository{database: database}
}

// Save lets the unique email index decide, since check-then-create races.
func (userRepository *UserRepository) Save(
	executionContext context.Context, user entities.User,
) (entities.User, error) {
	result := userRepository.database.WithContext(executionContext).Create(&user)
	if userRepository.isEmailAlreadyHeld(result.Error) {
		return entities.User{}, domains.EmailAlreadyRegistered(user.Email)
	}
	if result.Error != nil {
		return entities.User{}, fmt.Errorf("save user: %w", result.Error)
	}

	return user, nil
}

// FindOneByEmail uses the address as given; normalization is the domain's job.
func (userRepository *UserRepository) FindOneByEmail(
	executionContext context.Context, email string,
) (entities.User, error) {
	user := entities.User{}

	// A string condition is used because GORM drops zero-valued struct fields, so an empty address would match any user.
	result := userRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "email", Value: email}).
		First(&user)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.User{}, domains.ErrUserNotFound
	}
	if result.Error != nil {
		return entities.User{}, fmt.Errorf("find user by email: %w", result.Error)
	}

	return user, nil
}

func (userRepository *UserRepository) FindOne(
	executionContext context.Context, id uint,
) (entities.User, error) {
	user := entities.User{}

	result := userRepository.database.WithContext(executionContext).First(&user, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return entities.User{}, domains.ErrUserNotFound
	}
	if result.Error != nil {
		return entities.User{}, fmt.Errorf("find user: %w", result.Error)
	}

	return user, nil
}

// isEmailAlreadyHeld detects the email index specifically; other constraint violations stay storage failures.
func (userRepository *UserRepository) isEmailAlreadyHeld(writeError error) bool {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if !isPostgresError {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == UserEmailIndex
}

// ChangePasswordProof replaces the proof and revokes all open sessions in one transaction; already-ended sessions keep their original end time.
func (userRepository *UserRepository) ChangePasswordProof(
	executionContext context.Context, userID uint, newPasswordProof string,
) error {
	return userRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// Select names all three columns so the cleared lockout fields (zero values) are actually written.
			replaced := transaction.Model(&entities.User{}).
				Where(clause.Eq{Column: "id", Value: userID}).
				Select("password_proof", "failed_sign_in_count", "locked_until").
				Updates(entities.User{PasswordProof: newPasswordProof})
			if replaced.Error != nil {
				return fmt.Errorf("change password proof: %w", replaced.Error)
			}
			// No rows means no such user; returning an error rolls back the session revocation.
			if replaced.RowsAffected == 0 {
				return domains.ErrUserNotFound
			}

			revoked := transaction.Model(&entities.Session{}).
				Where(clause.Eq{Column: "user_id", Value: userID}).
				Where(clause.Eq{Column: "revoked_at", Value: nil}).
				Update("revoked_at", gorm.Expr("now()"))
			if revoked.Error != nil {
				return fmt.Errorf("revoke sessions after password change: %w", revoked.Error)
			}

			return nil
		})
}

// SaveSignInLockoutState always writes both columns together, including the nil that clears the lock.
func (userRepository *UserRepository) SaveSignInLockoutState(
	executionContext context.Context,
	userID uint,
	observedFailedSignInCount int,
	state vo.SignInLockoutStateVo,
) error {
	// Select forces the zero values to be written, and the guard on the previously read streak rejects concurrent attempts instead of overwriting them.
	saved := userRepository.database.WithContext(executionContext).
		Model(&entities.User{}).
		Where(clause.Eq{Column: "id", Value: userID}).
		Where(clause.Eq{Column: "failed_sign_in_count", Value: observedFailedSignInCount}).
		Select("failed_sign_in_count", "locked_until").
		Updates(entities.User{
			FailedSignInCount: state.FailedSignInCount,
			LockedUntil:       state.LockedUntil,
		})
	if saved.Error != nil {
		return fmt.Errorf("save sign in lockout state: %w", saved.Error)
	}
	if saved.RowsAffected == 1 {
		return nil
	}

	// Distinguish a concurrent change from a missing user; a missing user must not be a silent no-op.
	if userRepository.database.WithContext(executionContext).
		Model(&entities.User{}).
		Where(clause.Eq{Column: "id", Value: userID}).
		Limit(1).
		Find(&entities.User{}).RowsAffected == 0 {
		return domains.ErrUserNotFound
	}

	return domains.ErrSignInLockoutStateStale
}

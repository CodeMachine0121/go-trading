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

// UserEmailIndex is the unique index on a user's email address. It is how an address
// that is already somebody's account is told apart from any other constraint on the
// table — the primary key included, which breaks when a restored dump leaves the
// identifier sequence behind and has nothing to do with anybody's address.
//
// The index name is repeated from the entity's tag because a struct tag cannot hold
// a constant. If the two ever drift, registering a taken address stops being
// reported as "somebody has that one" and starts being reported as a storage
// failure. It is exported so that the agreement between the two spellings is
// asserted by a test needing no database, rather than only by one that skips when
// there is none.
const UserEmailIndex = "idx_users_email"

// UserRepository stores the people this system recognises, in PostgreSQL.
type UserRepository struct {
	database *gorm.DB
}

func NewUserRepository(database *gorm.DB) *UserRepository {
	return &UserRepository{database: database}
}

// Save stores a new user, letting the unique index on the email address decide
// whether they may exist. Asking first and creating afterwards would let two
// registrations arriving at once both find the address free.
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

// FindOneByEmail returns the user whose account is this address. The address is used
// exactly as handed in: deciding that two spellings are the same address is the
// domain's job, and doing it again here would be a second opinion that can disagree.
func (userRepository *UserRepository) FindOneByEmail(
	executionContext context.Context, email string,
) (entities.User, error) {
	user := entities.User{}

	// The condition is spelled out rather than given as a struct, because GORM drops
	// zero-valued struct fields — so an empty address would become no condition at
	// all, and this would hand back whichever user happens to be first in the table
	// for their password to be checked against. Nothing reaches here with an empty
	// address today; this is so that nothing can start to.
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

// FindOne returns the user carrying this identifier.
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

// isEmailAlreadyHeld says whether this write broke the email index specifically.
// Every other broken constraint stays a storage failure: answering "somebody has
// that address" for a clash the address had no part in would send whoever reads it
// hunting for an account that does not exist.
func (userRepository *UserRepository) isEmailAlreadyHeld(writeError error) bool {
	postgresError, isPostgresError := errors.AsType[*pgconn.PgError](writeError)
	if !isPostgresError {
		return false
	}

	return postgresError.Code == uniqueViolationCode &&
		postgresError.ConstraintName == UserEmailIndex
}

// ChangePasswordProof replaces a user's password proof and ends every session they
// still have open, both inside one transaction.
//
// One transaction is the whole point. The half-done state this avoids is not merely
// untidy — it is the new password in force while sessions opened with the old one
// keep working, which is precisely the situation somebody changes their password to
// end. A caller sequencing two writes would have to know which order avoids it, and
// would still be exposed to the second one failing.
//
// Sessions already ended keep the moment they were ended, for the same reason
// RevokeChain leaves them alone: the first answer to "when did this stop" is the
// true one, and overwriting it erases the trail.
func (userRepository *UserRepository) ChangePasswordProof(
	executionContext context.Context, userID uint, newPasswordProof string,
) error {
	return userRepository.database.WithContext(executionContext).Transaction(
		func(transaction *gorm.DB) error {
			// The lock goes in the same statement as the proof rather than a
			// second one: somebody who changed their password has already proved
			// who they are, and there is no instant in between where the new
			// password works but the door is still shut.
			// Select names all three so that the two being cleared are written
			// rather than skipped: an update from a struct leaves zero values
			// alone, and "no wrong passwords" and "not shut" are both zero.
			replaced := transaction.Model(&entities.User{}).
				Where(clause.Eq{Column: "id", Value: userID}).
				Select("password_proof", "failed_sign_in_count", "locked_until").
				Updates(entities.User{PasswordProof: newPasswordProof})
			if replaced.Error != nil {
				return fmt.Errorf("change password proof: %w", replaced.Error)
			}
			// No rows updated means there is nobody by that identifier. Returning
			// an error rolls the transaction back, so a change that reached nobody
			// never gets to sign anybody out either.
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

// SaveSignInLockoutState records what one attempt at signing in left behind.
//
// Both columns are written every time, including the nil that clears the lock. A
// partial write — the count without the moment, or the other way round — would leave
// a row saying two things that cannot both be true, and nothing downstream could tell
// which half to believe.
func (userRepository *UserRepository) SaveSignInLockoutState(
	executionContext context.Context, userID uint, state vo.SignInLockoutStateVo,
) error {
	// Both columns are named in Select so that clearing them actually clears them:
	// an update from a struct skips zero values, and both halves of "nothing held
	// against this account" are zero.
	saved := userRepository.database.WithContext(executionContext).
		Model(&entities.User{}).
		Where(clause.Eq{Column: "id", Value: userID}).
		Select("failed_sign_in_count", "locked_until").
		Updates(entities.User{
			FailedSignInCount: state.FailedSignInCount,
			LockedUntil:       state.LockedUntil,
		})
	if saved.Error != nil {
		return fmt.Errorf("save sign in lockout state: %w", saved.Error)
	}
	// Nobody by that identifier is not a quiet no-op here. The caller is the sign-in
	// flow recording what just happened, and a record that reached nobody means the
	// lock silently does not exist for that account.
	if saved.RowsAffected == 0 {
		return domains.ErrUserNotFound
	}

	return nil
}

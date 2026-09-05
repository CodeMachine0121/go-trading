package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
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

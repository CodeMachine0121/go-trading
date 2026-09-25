package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_user_repository.go -destination=mocks/mock_i_user_repository.go -package=mocks

// IUserRepository enforces email uniqueness at write time so concurrent registrations cannot create duplicates.
type IUserRepository interface {
	// Save returns ErrEmailAlreadyRegistered when the address is taken.
	Save(executionContext context.Context, user entities.User) (entities.User, error)
	// FindOneByEmail expects an already-normalised address and returns ErrUserNotFound when absent.
	FindOneByEmail(executionContext context.Context, email string) (entities.User, error)
	// FindOne returns ErrUserNotFound when absent.
	FindOne(executionContext context.Context, id uint) (entities.User, error)
	// ChangePasswordProof replaces the proof, ends all open sessions and clears any sign-in lock in one transaction, so old sessions never outlive a password change; ErrUserNotFound when absent.
	ChangePasswordProof(executionContext context.Context, userID uint, newPasswordProof string) error
	// SaveSignInLockoutState writes the whole lockout standing only while the row's streak still equals observedFailedSignInCount, else ErrSignInLockoutStateStale, so parallel guesses cannot share one count.
	// ErrUserNotFound when the user does not exist.
	SaveSignInLockoutState(
		executionContext context.Context,
		userID uint,
		observedFailedSignInCount int,
		state vo.SignInLockoutStateVo,
	) error
}

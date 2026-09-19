package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_user_repository.go -destination=mocks/mock_i_user_repository.go -package=mocks

// IUserRepository stores and retrieves the people this system recognises.
//
// Whether an email address is already somebody's account is answered here rather
// than by whoever calls. It is a fact about what is stored, and a caller that asked
// first and created afterwards would be reading state it then acts on separately —
// which two registrations arriving at once turn into two accounts on one address.
type IUserRepository interface {
	// Save stores a new user and returns them as stored, identifier and times filled
	// in. An address another user already holds is refused with
	// ErrEmailAlreadyRegistered.
	Save(executionContext context.Context, user entities.User) (entities.User, error)
	// FindOneByEmail returns the user whose account is this address, or
	// ErrUserNotFound. The address is expected already normalised — this interface
	// looks things up, it does not decide what two spellings mean.
	FindOneByEmail(executionContext context.Context, email string) (entities.User, error)
	// FindOne returns the user carrying this identifier, or ErrUserNotFound.
	FindOne(executionContext context.Context, id uint) (entities.User, error)
	// ChangePasswordProof replaces a user's password proof and, in the same
	// transaction, ends every session they still have open. A user this identifier
	// matches nobody is refused with ErrUserNotFound.
	//
	// The two happen together, and that is most of why this is one method rather
	// than two calls somebody sequences. Between them lies exactly the state this
	// feature exists to prevent: the new password in force while the sessions opened
	// with the old one keep working — which is the person the password was changed
	// because of, still inside, with the screen saying it was changed.
	//
	// Sequenced by a caller, that window's absence would depend on the caller
	// remembering the order, and on the next person to add a third step knowing why
	// the order was that way. Written here, there is no order to get wrong.
	//
	// The sessions are the user's own — the schema already says so, cascading them
	// on delete — so this is one aggregate's invariant, not two aggregates being
	// coordinated.
	//
	// Clearing any sign-in lock joins the same transaction, for the same reason the
	// sessions do. Somebody changing their password has already proved they are the
	// account holder — they needed a valid proof of identity to get this far — so
	// keeping them shut out afterwards protects nothing. Left to a second call,
	// "password changed but still locked out" becomes a state that genuinely exists,
	// and whether anybody ever sees it depends on the next person to add a third
	// step knowing why the order was what it was.
	ChangePasswordProof(executionContext context.Context, userID uint, newPasswordProof string) error
	// SaveSignInLockoutState records what one attempt at signing in left behind: how
	// many wrong passwords this account has in a row, and until when it is shut. A
	// user this identifier matches nobody is refused with ErrUserNotFound.
	//
	// It writes the whole standing rather than nudging a counter, because the
	// account's next state is worked out in one place by the domain. A store that
	// offered "add one" and "clear" separately would be offering the two halves of
	// a decision that is only ever made whole.
	SaveSignInLockoutState(
		executionContext context.Context, userID uint, state vo.SignInLockoutStateVo,
	) error
}

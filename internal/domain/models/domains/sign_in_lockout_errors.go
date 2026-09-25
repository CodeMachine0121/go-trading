package domains

import (
	"errors"
	"fmt"
	"time"
)

// ErrSignInLocked is deliberately distinct from ErrCredentialsRejected so a user with the right password is not told to retype it during the lock.
var ErrSignInLocked = errors.New("這個帳號因為連續登入失敗已被鎖住")

// ErrSignInLockoutStateStale signals that a concurrent attempt changed the streak between read and write; without this guard parallel guesses would all count as one.
var ErrSignInLockoutStateStale = errors.New("這次登入的計數已被另一次登入改寫")

// SignInLockedError carries the lock's end so callers don't re-derive the lock duration from settings.
type SignInLockedError struct {
	LockedUntil time.Time
}

func (signInLockedError SignInLockedError) Error() string {
	// UTC so the message doesn't depend on the process timezone; callers convert to the viewer's.
	return fmt.Sprintf("%s，%s 之後才能再試",
		ErrSignInLocked.Error(),
		signInLockedError.LockedUntil.UTC().Format(time.RFC3339))
}

// Is lets errors.Is match ErrSignInLocked while errors.As still reaches the lock end.
func (signInLockedError SignInLockedError) Is(target error) bool {
	return target == ErrSignInLocked
}

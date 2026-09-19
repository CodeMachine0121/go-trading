package domains

import (
	"errors"
	"fmt"
	"time"
)

// ErrSignInLocked is what the door says to an account that has collected too many
// wrong passwords in a row.
//
// It is deliberately not ErrCredentialsRejected. That one means "the pair did not
// match", and somebody acting on it types their password again — which, for the
// week this lock lasts, is the one thing that cannot change their situation. Told
// the same sentence either way, the person whose password was right all along would
// spend the week believing they had forgotten it.
var ErrSignInLocked = errors.New("這個帳號因為連續登入失敗已被鎖住")

// ErrSignInLockoutStateStale says somebody else counted an attempt against this
// account between it being read and it being written.
//
// It exists because the streak is read, added to in memory, and written back as a
// whole number — three moments with a deliberately slow password comparison in the
// middle. Without a guard, attempts fired at one address in parallel all read the
// same number and all write the same number, so a hundred guesses cost one, and the
// threshold is never reached. That is not a rare collision between two people
// mistyping at once; it is the cheapest way to defeat this lock, and it is available
// to exactly the tireless machine the lock exists for.
//
// The sign-in flow answers it by reading the row again and counting against what
// the row actually says now. It reaches a caller only when looking again has run
// out of tries against a row that is still not shut — which is the one case where
// an attempt really has gone uncounted, and saying nothing would leave the lock
// short of the guesses that happened.
var ErrSignInLockoutStateStale = errors.New("這次登入的計數已被另一次登入改寫")

// SignInLockedError is that refusal carrying the moment it ends.
//
// The moment travels inside the error for the same reason the activation
// instruction does: the layer that has to say it is the last layer that should know
// how long a lock lasts. A caller reading a duration out of a setting to build this
// sentence would be a second copy of a decision the domain already made.
type SignInLockedError struct {
	LockedUntil time.Time
}

func (signInLockedError SignInLockedError) Error() string {
	// UTC, because the moment comes back from storage wearing whatever timezone the
	// process happens to run in. That would make the same lock read differently on a
	// laptop and on the cluster, for no reason anybody chose. The caller showing this
	// to a person converts it to *their* timezone, which is the only local time that
	// means anything here.
	return fmt.Sprintf("%s，%s 之後才能再試",
		ErrSignInLocked.Error(),
		signInLockedError.LockedUntil.UTC().Format(time.RFC3339))
}

// Is lets callers that only want to know which refusal this is keep writing
// errors.Is, while the ones that need the moment reach for errors.As.
func (signInLockedError SignInLockedError) Is(target error) bool {
	return target == ErrSignInLocked
}

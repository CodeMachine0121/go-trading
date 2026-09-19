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

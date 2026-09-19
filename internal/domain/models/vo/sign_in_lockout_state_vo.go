package vo

import "time"

// SignInLockoutStateVo is what one account's standing with the door looks like after
// an attempt: how many wrong passwords it has collected in a row, and until when it
// is shut.
//
// LockedUntil is a pointer rather than a zero time because "not shut" and "was shut,
// and that has passed" are different facts and the store has to keep them apart.
// A zero time would make every account that has never been locked indistinguishable
// from one whose lock ended at the beginning of the epoch.
type SignInLockoutStateVo struct {
	FailedSignInCount int
	// LockedUntil is nil when the account is not shut.
	LockedUntil *time.Time
}

package vo

import "time"

// SignInLockoutStateVo is an account's consecutive failure count and lock expiry after an attempt.
type SignInLockoutStateVo struct {
	FailedSignInCount int
	// LockedUntil is nil when never locked, distinct from a lock that has already passed.
	LockedUntil *time.Time
}

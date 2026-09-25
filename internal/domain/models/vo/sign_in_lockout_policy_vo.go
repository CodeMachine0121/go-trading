package vo

import "time"

// SignInLockoutPolicyVo is how many consecutive wrong passwords lock an account and for how long; it is configurable so tests need not wait out a real lock.
type SignInLockoutPolicyVo struct {
	// FailureThreshold is inclusive: the attempt that reaches it is itself refused.
	FailureThreshold int
	// LockoutDuration counts from the locking attempt and is never extended by later ones.
	LockoutDuration time.Duration
}

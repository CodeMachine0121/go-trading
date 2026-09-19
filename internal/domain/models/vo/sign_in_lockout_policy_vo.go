package vo

import "time"

// SignInLockoutPolicyVo is how tired the sign-in door is allowed to get: how many
// wrong passwords in a row shut an account, and for how long.
//
// It is a setting rather than a pair of constants for the same reason the token
// lifetimes are: a test that wanted to watch a week-long lock expire would otherwise
// have to wait a week. Neither half is a key, so both have defaults and a console
// with nothing configured still starts with the door locked the way it should be.
type SignInLockoutPolicyVo struct {
	// FailureThreshold is how many consecutive wrong passwords shut the account.
	// The attempt that reaches it is itself refused — there is no moment where the
	// count is at the threshold and somebody is still let through.
	FailureThreshold int
	// LockoutDuration is how long the account stays shut, counted from the attempt
	// that shut it and never extended by the attempts that follow.
	LockoutDuration time.Duration
}

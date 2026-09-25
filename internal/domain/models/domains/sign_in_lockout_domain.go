package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SignInLockoutDomain owns every lockout rule: whether an attempt is refused before the password is checked, and the account's state after a failure or success.
type SignInLockoutDomain struct {
	user   entities.User
	policy vo.SignInLockoutPolicyVo
	now    time.Time
}

// NewSignInLockoutDomain takes now explicitly so both lock edges are testable.
func NewSignInLockoutDomain(
	user entities.User, policy vo.SignInLockoutPolicyVo, now time.Time,
) SignInLockoutDomain {
	return SignInLockoutDomain{user: user, policy: policy, now: now}
}

// Refusal returns nil or the lock error in one call, so callers cannot check "locked" without also getting the message.
func (signInLockoutDomain SignInLockoutDomain) Refusal() error {
	if !signInLockoutDomain.locked() {
		return nil
	}

	return SignInLockedError{LockedUntil: *signInLockoutDomain.user.LockedUntil}
}

// AfterFailure counts from zero again after an expired lock, so someone who sat out a lock gets the full threshold back rather than one attempt.
func (signInLockoutDomain SignInLockoutDomain) AfterFailure() vo.SignInLockoutStateVo {
	failedSignInCount := signInLockoutDomain.user.FailedSignInCount + 1
	if signInLockoutDomain.lockServed() {
		failedSignInCount = 1
	}

	if failedSignInCount < signInLockoutDomain.policy.FailureThreshold {
		return vo.SignInLockoutStateVo{FailedSignInCount: failedSignInCount}
	}

	lockedUntil := signInLockoutDomain.now.Add(signInLockoutDomain.policy.LockoutDuration).UTC()

	return vo.SignInLockoutStateVo{
		FailedSignInCount: failedSignInCount,
		LockedUntil:       &lockedUntil,
	}
}

// AfterSuccess ends any streak regardless of length.
func (signInLockoutDomain SignInLockoutDomain) AfterSuccess() vo.SignInLockoutStateVo {
	return vo.SignInLockoutStateVo{FailedSignInCount: 0, LockedUntil: nil}
}

// locked treats the lock's end instant itself as already open.
func (signInLockoutDomain SignInLockoutDomain) locked() bool {
	if signInLockoutDomain.user.LockedUntil == nil {
		return false
	}

	return signInLockoutDomain.now.Before(*signInLockoutDomain.user.LockedUntil)
}

// lockServed is deliberately not "not locked": a never-locked account's streak must keep climbing.
func (signInLockoutDomain SignInLockoutDomain) lockServed() bool {
	return signInLockoutDomain.user.LockedUntil != nil && !signInLockoutDomain.locked()
}

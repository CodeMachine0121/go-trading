package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SignInLockoutDomain is one account's standing with a door that gets tired: whether
// this attempt is refused before the password is even looked at, and what the account
// looks like once the attempt is over.
//
// Everything this feature knows lives here. Which instant counts as open, whether a
// fresh failure counts up from zero or from the number left behind by a lock that has
// since expired, and why an account already shut is never counted up at all — all of
// it is answered by the three methods below rather than by the sign-in flow.
//
// That matters because signing in already has three orderings that are deliberate and
// each carry a written reason. A fourth concern arriving as six lines about counts and
// timestamps would have the next person read all of them to work out where their
// change belongs. Arriving as an object, it is two questions asked in the obvious
// places, and lengthening the lock in steps later is a change to this file alone.
type SignInLockoutDomain struct {
	user   entities.User
	policy vo.SignInLockoutPolicyVo
	now    time.Time
}

// NewSignInLockoutDomain reads one account's standing from the row that holds it, the
// policy that says how tired the door gets, and the moment this attempt is happening.
//
// The moment is passed in rather than read here because everything else in this system
// that depends on the time takes it from the one clock the tests can wind — a lock
// that read the wall clock could not be tested at either of its two edges.
func NewSignInLockoutDomain(
	user entities.User, policy vo.SignInLockoutPolicyVo, now time.Time,
) SignInLockoutDomain {
	return SignInLockoutDomain{user: user, policy: policy, now: now}
}

// Refusal is what this attempt gets before its password is looked at, or nil if it
// gets nothing and the sign-in should carry on.
//
// It answers with an error rather than with a bool beside a separate error-builder
// because those are one question asked twice, and a question asked twice is one a
// caller can ask half of. Here "not locked" and "locked, and this is the sentence"
// are two outcomes of a single call, so there is no half to miss.
func (signInLockoutDomain SignInLockoutDomain) Refusal() error {
	if !signInLockoutDomain.locked() {
		return nil
	}

	return SignInLockedError{LockedUntil: *signInLockoutDomain.user.LockedUntil}
}

// AfterFailure is what the account looks like once this wrong password is counted.
//
// The count it adds to is the stored one, except after a lock that has run out. Such
// a lock leaves its final count behind in the row, and counting on from there would
// shut the account again on the first mistake after the week ended — somebody who sat
// out their lock would get one attempt back, not three. Serving the lock is what ends
// the streak, so the count starts over.
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

// AfterSuccess is what the account looks like once a right password has been
// accepted: nothing held against it, and no lock.
//
// Reaching for the threshold at all would be wrong here. Getting in is the end of
// any streak whatever its length, which is the whole of what "consecutive" promises.
func (signInLockoutDomain SignInLockoutDomain) AfterSuccess() vo.SignInLockoutStateVo {
	return vo.SignInLockoutStateVo{FailedSignInCount: 0, LockedUntil: nil}
}

// locked is whether the door is shut at this moment.
//
// The moment the lock names is itself open: a lock that ends at eight o'clock is over
// at eight o'clock, not at one past. Written as "now is not before then" rather than
// "now is after then" for exactly that boundary, which is the difference between the
// two comparisons and the only reason to prefer one.
//
// It is private and used by both public answers above, which is the only reason it is
// a method rather than written twice.
func (signInLockoutDomain SignInLockoutDomain) locked() bool {
	if signInLockoutDomain.user.LockedUntil == nil {
		return false
	}

	return signInLockoutDomain.now.Before(*signInLockoutDomain.user.LockedUntil)
}

// lockServed is whether this account was shut and has since sat the whole lock out.
//
// It is deliberately not "not locked": an account that has never been shut has no
// lock to have served, and its count is a plain streak that must keep climbing.
// Collapsing the two would reset the count on every ordinary second mistake, and the
// third wrong password would never arrive.
func (signInLockoutDomain SignInLockoutDomain) lockServed() bool {
	return signInLockoutDomain.user.LockedUntil != nil && !signInLockoutDomain.locked()
}

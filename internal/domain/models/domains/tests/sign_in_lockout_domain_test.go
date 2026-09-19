package domains_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tiring is the shipped policy rather than convenient small numbers, so that a test
// named "the third wrong password" is about the third one here too.
var tiring = vo.SignInLockoutPolicyVo{
	FailureThreshold: 3,
	LockoutDuration:  7 * 24 * time.Hour,
}

// attemptMoment is when every attempt below happens, and aWeekOn is what the policy
// makes of it. Both are written out rather than computed, so that asserting one is
// asserting the requirement instead of repeating the code's own arithmetic.
var (
	attemptMoment = time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	aWeekOn       = time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC)
)

// anAccount is one row's standing with the door, carrying only what decides the
// outcome — the streak and the moment it is shut until.
func anAccount(failedSignInCount int, lockedUntil *time.Time) entities.User {
	return entities.User{ID: 7, Email: "james@example.com", FailedSignInCount: failedSignInCount,
		LockedUntil: lockedUntil}
}

func shutUntil(moment time.Time) *time.Time {
	return &moment
}

func TestSignInLockoutRefusesAnAttemptOnlyWhileTheAccountIsStillShut(t *testing.T) {
	testCases := []struct {
		name              string
		failedSignInCount int
		lockedUntil       *time.Time
		expectedRefused   bool
	}{
		{
			name:              "an account nobody has ever shut lets the attempt through",
			failedSignInCount: 0,
			lockedUntil:       nil,
			expectedRefused:   false,
		},
		{
			name:              "an account with a streak but no lock lets the attempt through",
			failedSignInCount: 2,
			lockedUntil:       nil,
			expectedRefused:   false,
		},
		{
			name:              "an account shut until later is refused",
			failedSignInCount: 3,
			lockedUntil:       shutUntil(attemptMoment.Add(72 * time.Hour)),
			expectedRefused:   true,
		},
		{
			// The moment the lock names is itself open. A lock that ends at eight
			// o'clock is over at eight o'clock, not at one past.
			name:              "an account whose lock ends exactly now is let through",
			failedSignInCount: 3,
			lockedUntil:       shutUntil(attemptMoment),
			expectedRefused:   false,
		},
		{
			name:              "an account with one second of lock left is refused",
			failedSignInCount: 3,
			lockedUntil:       shutUntil(attemptMoment.Add(time.Second)),
			expectedRefused:   true,
		},
		{
			name:              "an account whose lock has run out is let through",
			failedSignInCount: 3,
			lockedUntil:       shutUntil(attemptMoment.Add(-time.Second)),
			expectedRefused:   false,
		},
		{
			// Nobody holding the address arrives here as a zero-valued row. It has
			// to pass, or an unregistered address would be refused differently from
			// a wrong password — which is the one distinction this door never draws.
			name:              "an address that is nobody's account is let through",
			failedSignInCount: 0,
			lockedUntil:       nil,
			expectedRefused:   false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lockout := domains.NewSignInLockoutDomain(
				anAccount(testCase.failedSignInCount, testCase.lockedUntil), tiring, attemptMoment)

			refusal := lockout.Refusal()

			if !testCase.expectedRefused {
				assert.NoError(t, refusal)
				return
			}
			require.Error(t, refusal)
			assert.ErrorIs(t, refusal, domains.ErrSignInLocked)
		})
	}
}

func TestSignInLockoutSaysWhenARefusedAttemptMayTryAgain(t *testing.T) {
	lockout := domains.NewSignInLockoutDomain(
		anAccount(3, shutUntil(aWeekOn)), tiring, attemptMoment)

	refusal := lockout.Refusal()

	// The moment travels inside the error so that the layer saying the sentence
	// never has to know how long a lock lasts.
	var locked domains.SignInLockedError
	require.ErrorAs(t, refusal, &locked)
	assert.Equal(t, aWeekOn, locked.LockedUntil)
	assert.Contains(t, refusal.Error(), "2026-01-08",
		"被鎖住的人要看得到什麼時候能再試，否則他只會每隔幾分鐘回來試一次")
}

func TestSignInLockoutCountsAWrongPasswordAndShutsTheAccountOnTheThird(t *testing.T) {
	testCases := []struct {
		name                string
		failedSignInCount   int
		lockedUntil         *time.Time
		expectedCount       int
		expectedLockedUntil *time.Time
	}{
		{
			name:                "the first wrong password is only counted",
			failedSignInCount:   0,
			lockedUntil:         nil,
			expectedCount:       1,
			expectedLockedUntil: nil,
		},
		{
			name:                "the second wrong password is only counted",
			failedSignInCount:   1,
			lockedUntil:         nil,
			expectedCount:       2,
			expectedLockedUntil: nil,
		},
		{
			name:                "the third wrong password shuts the account for a week",
			failedSignInCount:   2,
			lockedUntil:         nil,
			expectedCount:       3,
			expectedLockedUntil: shutUntil(aWeekOn),
		},
		{
			// Somebody who sat out a week would otherwise get one attempt back
			// rather than three: the count the expired lock left behind would carry
			// straight on into a fourth.
			name:                "the first wrong password after a served lock counts as one again",
			failedSignInCount:   3,
			lockedUntil:         shutUntil(attemptMoment.Add(-time.Second)),
			expectedCount:       1,
			expectedLockedUntil: nil,
		},
		{
			// If the threshold is ever lowered, the rows already above it must still
			// be shut. "Exactly equal" would leave them able to fail forever.
			name:                "a count already past the threshold still shuts the account",
			failedSignInCount:   5,
			lockedUntil:         nil,
			expectedCount:       6,
			expectedLockedUntil: shutUntil(aWeekOn),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lockout := domains.NewSignInLockoutDomain(
				anAccount(testCase.failedSignInCount, testCase.lockedUntil), tiring, attemptMoment)

			state := lockout.AfterFailure()

			assert.Equal(t, testCase.expectedCount, state.FailedSignInCount)
			if testCase.expectedLockedUntil == nil {
				assert.Nil(t, state.LockedUntil, "還沒到門檻就不該被鎖住")
				return
			}
			require.NotNil(t, state.LockedUntil)
			assert.Equal(t, *testCase.expectedLockedUntil, *state.LockedUntil)
		})
	}
}

func TestSignInLockoutClearsEverythingWhenTheRightPasswordArrives(t *testing.T) {
	testCases := []struct {
		name              string
		failedSignInCount int
		lockedUntil       *time.Time
	}{
		{name: "after a clean record", failedSignInCount: 0, lockedUntil: nil},
		{name: "after two wrong passwords", failedSignInCount: 2, lockedUntil: nil},
		{
			name:              "after a lock that has run out",
			failedSignInCount: 3,
			lockedUntil:       shutUntil(attemptMoment.Add(-time.Second)),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lockout := domains.NewSignInLockoutDomain(
				anAccount(testCase.failedSignInCount, testCase.lockedUntil), tiring, attemptMoment)

			state := lockout.AfterSuccess()

			// Getting in ends a streak of any length: that is the whole of what
			// "consecutive" promises.
			assert.Equal(t, 0, state.FailedSignInCount)
			assert.Nil(t, state.LockedUntil)
		})
	}
}

func TestSignInLockedErrorIsRecognisedWithoutUnwrappingIt(t *testing.T) {
	// Callers that only want to know which refusal this is keep writing errors.Is;
	// the one that needs the moment reaches for errors.As. Both have to work, or the
	// status code and the sentence would come apart.
	err := error(domains.SignInLockedError{LockedUntil: aWeekOn})

	assert.True(t, errors.Is(err, domains.ErrSignInLocked))
	assert.False(t, errors.Is(err, domains.ErrCredentialsRejected),
		"被鎖住與密碼不對是兩種拒絕，呼叫端要分得出來")
}

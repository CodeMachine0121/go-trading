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

// tiring is the shipped policy so "the third wrong password" means the real third.
var tiring = vo.SignInLockoutPolicyVo{
	FailureThreshold: 3,
	LockoutDuration:  7 * 24 * time.Hour,
}

// attemptMoment and aWeekOn are written out rather than computed so assertions check the requirement, not the code's arithmetic.
var (
	attemptMoment = time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	aWeekOn       = time.Date(2026, 1, 8, 8, 0, 0, 0, time.UTC)
)

// anAccount carries only the streak and the lock end.
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
			// A lock ending at eight is over at eight.
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
			// An unknown address arrives as a zero-valued row and must pass, so it isn't refused differently from a wrong password.
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
			// An expired lock's leftover count resets, so a served lock gives back three attempts rather than one.
			name:                "the first wrong password after a served lock counts as one again",
			failedSignInCount:   3,
			lockedUntil:         shutUntil(attemptMoment.Add(-time.Second)),
			expectedCount:       1,
			expectedLockedUntil: nil,
		},
		{
			// Counts above a lowered threshold still lock, which an exact-equality check would miss.
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

			// Success resets a streak of any length.
			assert.Equal(t, 0, state.FailedSignInCount)
			assert.Nil(t, state.LockedUntil)
		})
	}
}

func TestSignInLockedErrorIsRecognisedWithoutUnwrappingIt(t *testing.T) {
	// Both errors.Is and errors.As must work so the status code and message stay together.
	err := error(domains.SignInLockedError{LockedUntil: aWeekOn})

	assert.True(t, errors.Is(err, domains.ErrSignInLocked))
	assert.False(t, errors.Is(err, domains.ErrCredentialsRejected),
		"被鎖住與密碼不對是兩種拒絕，呼叫端要分得出來")
}

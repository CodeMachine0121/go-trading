package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordChangeDomainJudgesTheNewPasswordByTheSameRulesAsCreatingAnAccount(t *testing.T) {
	testCases := []struct {
		name        string
		newPassword string
	}{
		{
			name:        "an ordinary password is accepted",
			newPassword: "battery staple",
		},
		{
			name:        "exactly eight characters is enough",
			newPassword: "12345678",
		},
		{
			name:        "exactly seventy-two bytes is not too long",
			newPassword: strings.Repeat("密", 24),
		},
		{
			name:        "blanks are part of a password, so a password of blanks is a password",
			newPassword: "        ",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			passwordChange, err := domains.NewPasswordChangeDomain(dto.PasswordChangeDto{
				CurrentPassword: "correct horse",
				NewPassword:     testCase.newPassword,
			})

			require.NoError(t, err)
			assert.Equal(t, testCase.newPassword, passwordChange.NewPassword())
			assert.Equal(t, "correct horse", passwordChange.CurrentPassword())
		})
	}
}

func TestNewPasswordChangeDomainRefusesANewPasswordThatBreaksARule(t *testing.T) {
	testCases := []struct {
		name            string
		newPassword     string
		expectedMessage string
	}{
		{
			name:            "no password at all",
			newPassword:     "",
			expectedMessage: "必須給一組密碼",
		},
		{
			name:            "one character short of the minimum",
			newPassword:     "1234567",
			expectedMessage: "密碼至少要 8 個字元",
		},
		{
			name:            "three bytes over the maximum, which is one Chinese character over",
			newPassword:     strings.Repeat("密", 25),
			expectedMessage: "密碼長度上限為 72 個位元組",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewPasswordChangeDomain(dto.PasswordChangeDto{
				CurrentPassword: "correct horse",
				NewPassword:     testCase.newPassword,
			})

			require.ErrorIs(t, err, domains.ErrUserValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

// Reusing the same password would leave the user believing it changed.
func TestNewPasswordChangeDomainRefusesANewPasswordIdenticalToTheCurrentOne(t *testing.T) {
	_, err := domains.NewPasswordChangeDomain(dto.PasswordChangeDto{
		CurrentPassword: "correct horse",
		NewPassword:     "correct horse",
	})

	require.ErrorIs(t, err, domains.ErrUserValidation)
	assert.Contains(t, err.Error(), "新密碼不得與目前的密碼相同")
}

// Passwords are compared exactly, so a case-only difference is a real change.
func TestNewPasswordChangeDomainTreatsADifferenceInCaseAsADifferentPassword(t *testing.T) {
	passwordChange, err := domains.NewPasswordChangeDomain(dto.PasswordChangeDto{
		CurrentPassword: "correct horse",
		NewPassword:     "Correct horse",
	})

	require.NoError(t, err)
	assert.Equal(t, "Correct horse", passwordChange.NewPassword())
}

// The unacceptable-password refusal wins over the repeat, since it must be fixed either way.
func TestNewPasswordChangeDomainReportsAnUnacceptableNewPasswordBeforeItReportsARepeat(t *testing.T) {
	_, err := domains.NewPasswordChangeDomain(dto.PasswordChangeDto{
		CurrentPassword: "short",
		NewPassword:     "short",
	})

	require.ErrorIs(t, err, domains.ErrUserValidation)
	assert.Contains(t, err.Error(), "密碼至少要 8 個字元")
}

// The current password isn't judged by today's rules, which would lock out users whose password predates a rule change.
func TestNewPasswordChangeDomainDoesNotJudgeTheCurrentPassword(t *testing.T) {
	passwordChange, err := domains.NewPasswordChangeDomain(dto.PasswordChangeDto{
		CurrentPassword: "old",
		NewPassword:     "battery staple",
	})

	require.NoError(t, err)
	assert.Equal(t, "old", passwordChange.CurrentPassword())
}

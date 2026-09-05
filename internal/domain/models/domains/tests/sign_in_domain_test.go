package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSignInDomainLooksTheAccountUpByTheSameSpellingItWasStoredAs(t *testing.T) {
	testCases := []struct {
		name          string
		email         string
		expectedEmail string
	}{
		{
			name:          "an ordinary address",
			email:         "james@example.com",
			expectedEmail: "james@example.com",
		},
		{
			name:          "capitals and blanks are the same account",
			email:         "　JAMES@Example.com　",
			expectedEmail: "james@example.com",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			signInDomain, err := domains.NewSignInDomain(
				dto.SignInDto{Email: testCase.email, Password: "correct horse"})

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedEmail, signInDomain.Email())
			assert.Equal(t, "correct horse", signInDomain.Password())
		})
	}
}

func TestNewSignInDomainRefusesEverythingWithTheOneRefusalASignInHas(t *testing.T) {
	testCases := []struct {
		name      string
		signInDto dto.SignInDto
	}{
		{
			name:      "no address",
			signInDto: dto.SignInDto{Email: "", Password: "correct horse"},
		},
		{
			name:      "an address that is not one says nothing about whether it is registered",
			signInDto: dto.SignInDto{Email: "not-an-email", Password: "correct horse"},
		},
		{
			name:      "no password",
			signInDto: dto.SignInDto{Email: "james@example.com", Password: ""},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewSignInDomain(testCase.signInDto)

			require.ErrorIs(t, err, domains.ErrCredentialsRejected)
			assert.NotErrorIs(t, err, domains.ErrUserValidation,
				"登入不得告訴外面是哪一條規則沒過——那是在描述一個他沒有的帳號")
		})
	}
}

func TestNewSignInDomainSaysTheSameSentenceWhicheverHalfIsWrong(t *testing.T) {
	_, missingAddressError := domains.NewSignInDomain(
		dto.SignInDto{Email: "", Password: "correct horse"})
	_, malformedAddressError := domains.NewSignInDomain(
		dto.SignInDto{Email: "not-an-email", Password: "correct horse"})
	_, missingPasswordError := domains.NewSignInDomain(
		dto.SignInDto{Email: "james@example.com", Password: ""})

	assert.Equal(t, "電子郵件或密碼不正確", missingAddressError.Error())
	assert.Equal(t, missingAddressError.Error(), malformedAddressError.Error())
	assert.Equal(t, missingAddressError.Error(), missingPasswordError.Error())
}

// A password shorter than registering would allow is not refused here, and that is
// deliberate: the length rules say what a password may be set to, not whether it is
// the right one now. Refusing it here would answer "your password is too short" to
// somebody whose password is merely not ours — and would lock every existing account
// out the day the minimum length goes up.
func TestNewSignInDomainDoesNotJudgeThePasswordAgainstTheRulesForSettingOne(t *testing.T) {
	signInDomain, err := domains.NewSignInDomain(
		dto.SignInDto{Email: "james@example.com", Password: "1234567"})

	require.NoError(t, err)
	assert.Equal(t, "1234567", signInDomain.Password())
}

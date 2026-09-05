package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPasswordDomainAcceptsAPasswordWithinBothLimits(t *testing.T) {
	testCases := []struct {
		name     string
		password string
	}{
		{name: "an ordinary password", password: "correct horse"},
		{name: "exactly the fewest characters allowed", password: "12345678"},
		{
			name:     "blanks are part of a password, so they are neither trimmed nor refused",
			password: "        ",
		},
		{name: "exactly the most bytes allowed", password: strings.Repeat("a", 72)},
		{
			name:     "twenty-four Chinese characters are exactly the most bytes allowed",
			password: strings.Repeat("密", 24),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			passwordDomain, err := domains.NewPasswordDomain(testCase.password)

			require.NoError(t, err)
			assert.Equal(t, testCase.password, passwordDomain.Value(),
				"密碼一個字都不能被動到——動了它，今天設得起來的密碼明天就登不進去")
		})
	}
}

func TestNewPasswordDomainRefusesAPasswordOutsideEitherLimit(t *testing.T) {
	testCases := []struct {
		name             string
		password         string
		expectedFragment string
	}{
		{
			name:             "nothing at all",
			password:         "",
			expectedFragment: "必須給一組密碼",
		},
		{
			name:             "one character short of the fewest allowed",
			password:         "1234567",
			expectedFragment: "至少要 8 個字元",
		},
		{
			name:             "the fewest allowed counts characters, not bytes",
			password:         "密碼短",
			expectedFragment: "至少要 8 個字元",
		},
		{
			name:             "one byte past the most allowed",
			password:         strings.Repeat("a", 73),
			expectedFragment: "72",
		},
		{
			name:             "twenty-five Chinese characters are three bytes past the most allowed",
			password:         strings.Repeat("密", 25),
			expectedFragment: "72",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewPasswordDomain(testCase.password)

			require.ErrorIs(t, err, domains.ErrUserValidation)
			assert.Contains(t, err.Error(), testCase.expectedFragment)
		})
	}
}

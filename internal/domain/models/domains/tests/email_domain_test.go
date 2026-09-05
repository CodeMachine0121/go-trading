package domains_test

import (
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEmailDomainNormalisesTheAddressTheSameWayEveryTime(t *testing.T) {
	testCases := []struct {
		name          string
		email         string
		expectedValue string
	}{
		{
			name:          "an ordinary address is kept as it was written",
			email:         "james@example.com",
			expectedValue: "james@example.com",
		},
		{
			name:          "the blanks around an address are not part of it",
			email:         "  james@example.com  ",
			expectedValue: "james@example.com",
		},
		{
			name:          "capitals are the same address as lower case",
			email:         "James@Example.com",
			expectedValue: "james@example.com",
		},
		{
			name:          "full-width blanks are blanks too",
			email:         "　JAMES@Example.com　",
			expectedValue: "james@example.com",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			emailDomain, err := domains.NewEmailDomain(testCase.email)

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedValue, emailDomain.Value())
		})
	}
}

func TestNewEmailDomainRefusesAnAddressThatIsNotOne(t *testing.T) {
	testCases := []struct {
		name             string
		email            string
		expectedFragment string
	}{
		{
			name:             "nothing at all",
			email:            "",
			expectedFragment: "必須給一個電子郵件",
		},
		{
			name:             "only blanks says nothing was given, not that it is malformed",
			email:            "   ",
			expectedFragment: "必須給一個電子郵件",
		},
		{
			name:             "only full-width blanks",
			email:            "　",
			expectedFragment: "必須給一個電子郵件",
		},
		{
			name:             "a word is not an address",
			email:            "not-an-email",
			expectedFragment: "格式",
		},
		{
			name:             "nothing after the at sign",
			email:            "james@",
			expectedFragment: "格式",
		},
		{
			name:             "nothing before the at sign",
			email:            "@example.com",
			expectedFragment: "格式",
		},
		{
			name:             "a blank inside the address",
			email:            "james example@x.com",
			expectedFragment: "格式",
		},
		{
			name:             "an address book line is not an account",
			email:            "James Hsueh <james@example.com>",
			expectedFragment: "格式",
		},
		{
			name:             "an address carrying the one byte PostgreSQL will not hold",
			email:            "james@example.com\x00",
			expectedFragment: "空字元",
		},
		{
			name:             "an address longer than any address can be",
			email:            strings.Repeat("a", 320) + "@example.com",
			expectedFragment: "320",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewEmailDomain(testCase.email)

			require.ErrorIs(t, err, domains.ErrUserValidation)
			assert.Contains(t, err.Error(), testCase.expectedFragment)
		})
	}
}

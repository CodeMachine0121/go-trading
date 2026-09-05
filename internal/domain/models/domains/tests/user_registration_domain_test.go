package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aRegistration() dto.UserRegistrationDto {
	return dto.UserRegistrationDto{Email: "James@Example.com", Password: "correct horse"}
}

func TestNewUserRegistrationDomainBuildsTheRowThatWillBeStored(t *testing.T) {
	registration, err := domains.NewUserRegistrationDomain(aRegistration())
	require.NoError(t, err)

	user := registration.ToEntity("a-password-proof")

	assert.Equal(t, "james@example.com", user.Email, "存進去的是正規化之後的那一個拼法")
	assert.Equal(t, "a-password-proof", user.PasswordProof)
	assert.Equal(t, uint(0), user.ID, "還不存在的使用者不帶自己的識別碼")
}

func TestNewUserRegistrationDomainHandsThePasswordOnForTheOneUseItHas(t *testing.T) {
	registration, err := domains.NewUserRegistrationDomain(aRegistration())
	require.NoError(t, err)

	assert.Equal(t, "correct horse", registration.Password())
}

func TestNewUserRegistrationDomainRefusesEitherHalfAndReportsTheAddressFirst(t *testing.T) {
	testCases := []struct {
		name             string
		registrationDto  dto.UserRegistrationDto
		expectedFragment string
	}{
		{
			name: "an address that is not one",
			registrationDto: dto.UserRegistrationDto{
				Email: "not-an-email", Password: "correct horse"},
			expectedFragment: "格式",
		},
		{
			name: "a password below the fewest characters allowed",
			registrationDto: dto.UserRegistrationDto{
				Email: "james@example.com", Password: "short"},
			expectedFragment: "至少要 8 個字元",
		},
		{
			name: "both wrong reports the address, because that is what has to be fixed either way",
			registrationDto: dto.UserRegistrationDto{
				Email: "not-an-email", Password: "short"},
			expectedFragment: "格式",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewUserRegistrationDomain(testCase.registrationDto)

			require.ErrorIs(t, err, domains.ErrUserValidation)
			assert.Contains(t, err.Error(), testCase.expectedFragment)
		})
	}
}

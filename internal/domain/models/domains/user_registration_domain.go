package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// UserRegistrationDomain reuses the email and password models and adds only their order and
// the row conversion.
type UserRegistrationDomain struct {
	email    EmailDomain
	password PasswordDomain
}

// NewUserRegistrationDomain reports an invalid email before an invalid password, since the
// email is the account.
func NewUserRegistrationDomain(registrationDto dto.UserRegistrationDto) (UserRegistrationDomain, error) {
	email, emailError := NewEmailDomain(registrationDto.Email)
	if emailError != nil {
		return UserRegistrationDomain{}, emailError
	}

	password, passwordError := NewPasswordDomain(registrationDto.Password)
	if passwordError != nil {
		return UserRegistrationDomain{}, passwordError
	}

	return UserRegistrationDomain{email: email, password: password}, nil
}

func (userRegistrationDomain UserRegistrationDomain) Password() string {
	return userRegistrationDomain.password.Value()
}

// ToEntity takes the password hash as an argument because hashing is infrastructure;
// timestamps are set by the store.
func (userRegistrationDomain UserRegistrationDomain) ToEntity(passwordProof string) entities.User {
	return entities.User{
		Email:         userRegistrationDomain.email.Value(),
		PasswordProof: passwordProof,
	}
}

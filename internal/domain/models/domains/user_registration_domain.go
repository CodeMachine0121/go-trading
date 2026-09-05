package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

// UserRegistrationDomain holds one attempt to create a user and guarantees every
// rule about it passed. An instance only exists when both halves were acceptable, so
// there is no half-valid registration further in.
//
// The two halves already have models that know their own rules; this one borrows
// them rather than restating either list a second time. What it adds is the order
// they are judged in and a single place that turns the pair into a row.
type UserRegistrationDomain struct {
	email    EmailDomain
	password PasswordDomain
}

// NewUserRegistrationDomain judges the address first and the password second.
//
// The order is deliberate rather than incidental. When both are wrong, one of the
// two refusals has to be the one that comes back, and it should be the address: it
// is what the account *is*, so a person who got it wrong has to fix it whatever they
// then do about the password. Reporting the password first would have them change
// their password to be told their address was never valid.
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

// Password is the password as typed, for the one use it has: being turned into a
// proof. Nothing else in the system asks for it.
func (userRegistrationDomain UserRegistrationDomain) Password() string {
	return userRegistrationDomain.password.Value()
}

// ToEntity is this registration as it is stored, given the proof derived from its
// password.
//
// The proof arrives as an argument rather than being worked out here, because
// working it out is cryptography and the domain is not allowed to know any. What the
// domain does know is that a proof is the only form the password may be stored in —
// which is why this is the one way to get a user row, and why the row it builds has
// nowhere to put the password itself.
//
// The times are left alone: when a user was created is recorded where the saving
// happens, not claimed here.
func (userRegistrationDomain UserRegistrationDomain) ToEntity(passwordProof string) entities.User {
	return entities.User{
		Email:         userRegistrationDomain.email.Value(),
		PasswordProof: passwordProof,
	}
}

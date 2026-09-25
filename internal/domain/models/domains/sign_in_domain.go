package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// SignInDomain normalises the address like registration but refuses every problem with one identical error, so the refusal cannot be used to discover which addresses have accounts.
// Password length rules are deliberately not applied, since they govern what a password may be set to, not whether it matches.
type SignInDomain struct {
	email    string
	password string
}

// NewSignInDomain refuses blank or malformed input before storage is queried, so the store is never asked a question that reveals who is registered.
func NewSignInDomain(signInDto dto.SignInDto) (SignInDomain, error) {
	email, emailError := NewEmailDomain(signInDto.Email)
	if emailError != nil {
		return SignInDomain{}, ErrCredentialsRejected
	}

	if signInDto.Password == "" {
		return SignInDomain{}, ErrCredentialsRejected
	}

	return SignInDomain{email: email.Value(), password: signInDto.Password}, nil
}

// Email is trimmed and lowercased, exactly as stored.
func (signInDomain SignInDomain) Email() string {
	return signInDomain.email
}

// Password is as typed, only for checking against the stored proof.
func (signInDomain SignInDomain) Password() string {
	return signInDomain.password
}

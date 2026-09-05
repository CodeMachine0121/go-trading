package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// SignInDomain holds one attempt to sign in, with the address normalised the same
// way it was normalised when the account was created.
//
// It looks like UserRegistrationDomain and behaves nothing like it, which is the
// whole reason it is a second model rather than a reuse of the first.
//
// Registering judges content and says which rule was broken, because the person is
// deciding what their account will be and needs to know what to change. Signing in
// judges nothing and says one thing: the pair did not match. An address that is not
// an address, a password below the minimum length, a password that is simply wrong —
// all of them come back identically, because every distinction this refusal draws is
// a question somebody can ask it. "That is not a valid address" and "wrong password"
// together are enough to walk a list of addresses and find out which ones are
// accounts here.
//
// The length rules are deliberately not applied. They govern what a password may be
// set to, not whether it is the right one now; applying them here would answer "your
// password is too short" to somebody whose password is merely not ours, and would
// stop working the day the minimum length changes and existing accounts keep theirs.
type SignInDomain struct {
	email    string
	password string
}

// NewSignInDomain normalises the address and checks that both halves are there,
// refusing everything with the one refusal a failed sign-in has.
//
// Refusing here rather than further in matters for a reason beyond tidiness: a blank
// or malformed address never reaches storage, so the store is never asked a question
// whose answer is a fact about who is registered.
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

// Email is the address to look the account up by: trimmed and lowered, exactly as it
// was stored.
func (signInDomain SignInDomain) Email() string {
	return signInDomain.email
}

// Password is the password as typed, for the one use it has: being checked against
// the stored proof.
func (signInDomain SignInDomain) Password() string {
	return signInDomain.password
}

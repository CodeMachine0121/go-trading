package domains

import (
	"errors"
	"fmt"
)

// ErrUserValidation marks a registration whose content broke one of its rules. The
// wrapped message names the rule, so a caller reports it without knowing the list.
var ErrUserValidation = errors.New("user validation failed")

// ErrEmailAlreadyRegistered marks an email address another user already holds. It is
// deliberately not a validation failure: the content is fine, the address is taken,
// and the two lead a caller to do different things about it.
var ErrEmailAlreadyRegistered = errors.New("email already registered")

// ErrUserNotFound marks a user named by something that matches nobody. It never
// reaches a caller of the sign-in path — see ErrCredentialsRejected for why — and is
// only how the store says "there is nothing here" to the code above it.
var ErrUserNotFound = errors.New("user not found")

// ErrCredentialsRejected is what every failed sign-in says, whichever half was
// wrong.
//
// Its text is the whole message rather than a prefix somebody wraps, and that is the
// point: there is exactly one thing this system will ever say about a failed
// sign-in, so there is nothing to wrap it with. Two spellings of it — one for "no
// such address", one for "wrong password" — would hand out, for free, a way to ask
// which email addresses are registered here.
var ErrCredentialsRejected = errors.New("電子郵件或密碼不正確")

// ErrCurrentPasswordRejected is what changing a password says when the password
// given as the one in force is not the one in force.
//
// It is deliberately not ErrCredentialsRejected. That one is vague on purpose,
// because a stranger is asking and every distinction it draws is a way to find out
// which email addresses are accounts here. Nobody reaches this one without already
// having been recognised: they are asking about their own password, and the answer
// contains no list anybody could walk. Being vague here would only have somebody
// retyping an address that was never in question.
//
// It is also not ErrUserValidation. Both are "you typed something wrong", but they
// are wrong in different boxes, and the caller has to put the explanation under the
// right one — which it cannot do if the two arrive as the same kind of refusal.
var ErrCurrentPasswordRejected = errors.New("目前的密碼不正確")

// ErrAuthenticationRequired is what every rejected proof of identity says: missing,
// tampered with, expired, or pointing at somebody who is no longer here.
//
// Those four are one answer for the same reason as above — telling them apart tells
// whoever is asking something about a token they are not holding. And all four lead
// the holder to do exactly the same thing, which is sign in again.
var ErrAuthenticationRequired = errors.New("請重新登入")

// ErrAccessTokenUnavailable marks a system that cannot issue a proof of identity at
// all, because it has no key to sign one with. It is not a rejection of anything the
// caller sent: their password was right, and there is nothing they can change.
//
// It is a separate failure from every other because the alternative — issuing a
// token nobody signed — is a token anybody can forge. Refusing to sign is the only
// safe way to be missing a key.
var ErrAccessTokenUnavailable = errors.New("access token cannot be issued")

// EmailAlreadyRegistered is the refusal a caller reads when an email address is
// already somebody's account. It is worded here once so that the store, which is the
// only place that finds out, does not have to invent the sentence itself.
func EmailAlreadyRegistered(email string) error {
	return fmt.Errorf("%w: 電子郵件「%s」已經有人用了", ErrEmailAlreadyRegistered, email)
}

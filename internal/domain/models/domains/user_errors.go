package domains

import (
	"errors"
	"fmt"
)

// ErrUserValidation wraps a message naming the broken rule.
var ErrUserValidation = errors.New("user validation failed")

// ErrEmailAlreadyRegistered is kept apart from validation failures because the content is valid.
var ErrEmailAlreadyRegistered = errors.New("email already registered")

// ErrUserNotFound never reaches sign-in callers; it only signals an empty lookup internally.
var ErrUserNotFound = errors.New("user not found")

// ErrCredentialsRejected is the single, unwrapped message for every failed sign-in so
// registered emails cannot be probed.
var ErrCredentialsRejected = errors.New("電子郵件或密碼不正確")

// ErrCurrentPasswordRejected is specific, unlike ErrCredentialsRejected, because the caller
// is already authenticated, and is distinct from ErrUserValidation so the error lands under
// the right field.
var ErrCurrentPasswordRejected = errors.New("目前的密碼不正確")

// ErrAuthenticationRequired covers missing, tampered, expired and orphaned tokens alike so
// nothing leaks about a token and the remedy is always to sign in again.
var ErrAuthenticationRequired = errors.New("請重新登入")

// ErrAccessTokenUnavailable means no signing key is configured; refusing is the only safe
// alternative to issuing forgeable tokens.
var ErrAccessTokenUnavailable = errors.New("access token cannot be issued")

func EmailAlreadyRegistered(email string) error {
	return fmt.Errorf("%w: 電子郵件「%s」已經有人用了", ErrEmailAlreadyRegistered, email)
}

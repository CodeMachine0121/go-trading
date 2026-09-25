package domains

import "errors"

// ErrSessionNotFound never reaches a caller: renewing maps it to ErrAuthenticationRequired and sign-out treats it as success.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionAlreadyRotated reports the write-time check that a renewal proof works once, since two concurrent renewals can both read a valid session; callers treat it as proof reuse.
var ErrSessionAlreadyRotated = errors.New("session already rotated")

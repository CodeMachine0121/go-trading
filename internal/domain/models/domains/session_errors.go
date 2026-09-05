package domains

import "errors"

// ErrSessionNotFound marks a renewal proof that matches no stored session. Like
// ErrUserNotFound it never reaches a caller: renewing turns it into
// ErrAuthenticationRequired, and signing out treats it as success, because a session
// that is not there is a session nobody has to end.
var ErrSessionNotFound = errors.New("session not found")

// ErrSessionAlreadyRotated marks a rotation whose previous session had already been
// ended by the time the write ran.
//
// It exists because looking and then writing are two moments, and something can
// happen in between. Two renewals carrying the same proof both look, both see a
// session that is still good, and both go on to write — so "a renewal proof works
// once" cannot be a fact the reading establishes. It has to be established by the
// write itself, which is what this error reports failing.
//
// To a caller it means the same thing as finding an already-revoked session: the
// proof was used twice, and the chain has to go.
var ErrSessionAlreadyRotated = errors.New("session already rotated")

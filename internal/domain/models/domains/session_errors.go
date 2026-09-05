package domains

import "errors"

// ErrSessionNotFound marks a renewal proof that matches no stored session. Like
// ErrUserNotFound it never reaches a caller: renewing turns it into
// ErrAuthenticationRequired, and signing out treats it as success, because a session
// that is not there is a session nobody has to end.
var ErrSessionNotFound = errors.New("session not found")

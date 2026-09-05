package vo

import "time"

// SessionLifetimesVo is how long each half of a session lasts. Immutable, no
// behavior.
//
// The two live in one value because they are one decision made twice, not two
// unrelated settings. Shortening the access token's life is what shrinks the window
// in which a signed-out token still works; lengthening the renewal proof's is what
// buys not typing a password for weeks. Choosing one without looking at the other is
// how a system ends up with a nine-hour hole nobody meant to leave.
type SessionLifetimesVo struct {
	AccessToken  time.Duration
	RefreshToken time.Duration
}

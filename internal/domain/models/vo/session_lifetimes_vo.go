package vo

import "time"

// SessionLifetimesVo holds the access and refresh token lifetimes together because they must be chosen as one decision.
type SessionLifetimesVo struct {
	AccessToken  time.Duration
	RefreshToken time.Duration
}

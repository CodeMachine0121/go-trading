package vo

import "time"

// AccessTokenVo is an issued token paired with its expiry, so the expiry is never left to be guessed.
type AccessTokenVo struct {
	AccessToken string
	ExpiresAt   time.Time
}

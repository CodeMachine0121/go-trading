package dto

import "time"

// AccessTokenDto is what a successful sign-in hands back: the proof of identity
// itself, and the moment it stops being one.
//
// The expiry travels alongside the token rather than being left for the holder to
// read out of it. Reading it out would mean every holder having to understand the
// token's innards, and the whole point of the token is that only the system does.
type AccessTokenDto struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

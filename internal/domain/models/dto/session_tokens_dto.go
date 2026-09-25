package dto

import "time"

// SessionTokensDto returns expiries alongside tokens because the refresh token is opaque
// random data and the access token's contents are internal.
type SessionTokensDto struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	// RefreshToken is only ever returned here; only a non-reversible derivative is stored.
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

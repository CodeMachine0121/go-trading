package dto

// SessionRenewalDto carries only the refresh token, since renewal exists for when the access
// token has expired.
type SessionRenewalDto struct {
	RefreshToken string
}

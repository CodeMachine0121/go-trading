package dto

// SessionRenewalDto carries only the refresh token, since renewal exists for when the access
// token has expired.
type SessionRenewalDto struct {
	RefreshToken string
	// ConnectorClientIdentifier is empty for web renewals; a session renews only through the channel that opened it.
	ConnectorClientIdentifier string
}

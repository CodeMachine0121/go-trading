package vo

// RefreshTokenVo pairs a freshly minted refresh token with its stored digest; the token cannot be recovered from the digest afterwards.
type RefreshTokenVo struct {
	Value  string
	Digest string
}

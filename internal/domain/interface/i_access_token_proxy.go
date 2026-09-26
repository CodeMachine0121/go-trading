package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

//go:generate go tool mockgen -source=i_access_token_proxy.go -destination=mocks/mock_i_access_token_proxy.go -package=mocks

// IAccessTokenProxy issues access tokens and reads them back.
type IAccessTokenProxy interface {
	// Issue signs the claims; the expiry is supplied because session length is a business rule, and the audience is written only when present.
	// With no signing key it returns ErrAccessTokenUnavailable rather than an unsigned token.
	Issue(claims vo.AccessTokenClaimsVo) (vo.AccessTokenVo, error)
	// ClaimsOf returns what a valid token asserts, audience included; any invalid token yields ErrAuthenticationRequired.
	ClaimsOf(accessToken string) (vo.AccessTokenClaimsVo, error)
	// UserIdentifiedBy returns the token's user; missing, tampered, expired or foreign-key tokens all yield ErrAuthenticationRequired.
	UserIdentifiedBy(accessToken string) (uint, error)
}

package _interface

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_access_token_proxy.go -destination=mocks/mock_i_access_token_proxy.go -package=mocks

// IAccessTokenProxy issues access tokens and reads them back.
type IAccessTokenProxy interface {
	// Issue signs a token for userID valid until expiresAt, which callers supply since session length is a business rule.
	// With no signing key it returns ErrAccessTokenUnavailable rather than an unsigned token.
	Issue(userID uint, expiresAt time.Time) (vo.AccessTokenVo, error)
	// UserIdentifiedBy returns the token's user; missing, tampered, expired or foreign-key tokens all yield ErrAuthenticationRequired.
	UserIdentifiedBy(accessToken string) (uint, error)
}

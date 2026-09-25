package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

//go:generate go tool mockgen -source=i_refresh_token_proxy.go -destination=mocks/mock_i_refresh_token_proxy.go -package=mocks

// IRefreshTokenProxy is separate from IPasswordProofProxy because refresh tokens are random (no need for slow hashing) and must be findable by an unsalted, deterministic digest.
type IRefreshTokenProxy interface {
	// Mint returns the value to hand out together with the digest to store, since the value exists only now.
	Mint() (vo.RefreshTokenVo, error)
	// DigestOf is deterministic so the session holding a token can be looked up.
	DigestOf(refreshToken string) string
}

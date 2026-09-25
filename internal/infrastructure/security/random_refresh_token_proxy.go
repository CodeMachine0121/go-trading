package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// RandomRefreshTokenProxy uses unsalted SHA-256 because tokens are 256 random bits (nothing to brute-force) and the digest must be deterministic to look the session up.
type RandomRefreshTokenProxy struct{}

func NewRandomRefreshTokenProxy() *RandomRefreshTokenProxy {
	return &RandomRefreshTokenProxy{}
}

// Mint's error is for future implementations; rand.Text never fails.
func (randomRefreshTokenProxy *RandomRefreshTokenProxy) Mint() (vo.RefreshTokenVo, error) {
	value := rand.Text()

	return vo.RefreshTokenVo{Value: value, Digest: digestOf(value)}, nil
}

func (randomRefreshTokenProxy *RandomRefreshTokenProxy) DigestOf(refreshToken string) string {
	return digestOf(refreshToken)
}

// digestOf is shared by minting and lookup so the two cannot drift.
func digestOf(refreshToken string) string {
	digest := sha256.Sum256([]byte(refreshToken))

	return hex.EncodeToString(digest[:])
}

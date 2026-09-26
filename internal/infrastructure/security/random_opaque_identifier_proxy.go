package security

import (
	"crypto/rand"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// RandomOpaqueIdentifierProxy mints 256 random bits and digests them like refresh tokens, since there is nothing to brute-force.
type RandomOpaqueIdentifierProxy struct{}

func NewRandomOpaqueIdentifierProxy() *RandomOpaqueIdentifierProxy {
	return &RandomOpaqueIdentifierProxy{}
}

// Mint's error is for future implementations; rand.Text never fails.
func (randomOpaqueIdentifierProxy *RandomOpaqueIdentifierProxy) Mint() (vo.OpaqueIdentifierVo, error) {
	value := rand.Text()

	return vo.OpaqueIdentifierVo{Value: value, Digest: digestOf(value)}, nil
}

func (randomOpaqueIdentifierProxy *RandomOpaqueIdentifierProxy) DigestOf(identifier string) string {
	return digestOf(identifier)
}

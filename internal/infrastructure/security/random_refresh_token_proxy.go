package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// RandomRefreshTokenProxy produces renewal proofs from cryptographic randomness and
// derives what is stored in their place with SHA-256.
//
// SHA-256 rather than the deliberately slow derivation used for passwords, and that
// choice is the whole design of this type:
//
//   - Slowness buys nothing here. It exists to make guessing expensive, and there is
//     nothing to guess: these values are not chosen by a person, they are 256 bits of
//     randomness. An attacker with the whole dictionary of human passwords has no
//     entry for any of them.
//   - Salt would break it outright. A password proof carries its own salt, which is
//     why finding a user by their proof is impossible and why sign-in looks the user
//     up by address instead. A session has no address to be looked up by — the proof
//     *is* the address — so the derivation has to be the same every time.
//
// Using bcrypt here would therefore get both halves wrong at once: every renewal
// would scan the whole table comparing row by row, and it would pay for protection
// against an attack that cannot happen.
type RandomRefreshTokenProxy struct{}

func NewRandomRefreshTokenProxy() *RandomRefreshTokenProxy {
	return &RandomRefreshTokenProxy{}
}

// Mint produces a new renewal proof and the digest to keep in its place.
//
// The value is produced by rand.Text, which is documented never to fail and to
// return enough randomness that guessing is not a strategy. The error in the
// signature is for the implementations that come after this one — a hardware
// security module has plenty to say about failing — and keeping it costs the caller
// one branch it has to write anyway.
func (randomRefreshTokenProxy *RandomRefreshTokenProxy) Mint() (vo.RefreshTokenVo, error) {
	value := rand.Text()

	return vo.RefreshTokenVo{Value: value, Digest: digestOf(value)}, nil
}

// DigestOf derives what would have been stored for this proof, so the session
// holding it can be found. The same proof always gives the same digest — which is
// the point, and is safe here for the reasons on the type.
func (randomRefreshTokenProxy *RandomRefreshTokenProxy) DigestOf(refreshToken string) string {
	return digestOf(refreshToken)
}

// digestOf is the one derivation, written once so that minting and looking up can
// never drift into two different answers — which would lock every holder out at the
// moment they tried to renew.
func digestOf(refreshToken string) string {
	digest := sha256.Sum256([]byte(refreshToken))

	return hex.EncodeToString(digest[:])
}

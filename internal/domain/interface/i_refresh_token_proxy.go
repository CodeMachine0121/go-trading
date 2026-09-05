package _interface

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

//go:generate go tool mockgen -source=i_refresh_token_proxy.go -destination=mocks/mock_i_refresh_token_proxy.go -package=mocks

// IRefreshTokenProxy produces renewal proofs and derives what is stored in their
// place.
//
// It is a separate capability from IPasswordProofProxy even though both turn a
// secret into something storable, because the two are solving opposite problems and
// the right answer to one is the wrong answer to the other:
//
//   - A password is chosen by a person, so it can be guessed from a list. The
//     defence is to make each guess expensive, and each stored proof carries its own
//     salt — which is exactly why a password cannot be looked up by its proof.
//   - A renewal proof is generated here from cryptographic randomness. There is no
//     list to guess from, so an expensive derivation buys nothing — and the proof
//     must be findable, which salt would make impossible.
//
// Putting them behind one interface would force one of the two to be wrong.
type IRefreshTokenProxy interface {
	// Mint produces a new renewal proof: the value to hand out, and the digest to
	// keep. Both come back together because the value exists for this moment only.
	Mint() (vo.RefreshTokenVo, error)
	// DigestOf derives what would have been stored for this proof, so that the
	// session holding it can be found. The same proof always gives the same digest.
	DigestOf(refreshToken string) string
}

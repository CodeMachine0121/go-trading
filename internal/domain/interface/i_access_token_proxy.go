package _interface

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_access_token_proxy.go -destination=mocks/mock_i_access_token_proxy.go -package=mocks

// IAccessTokenProxy issues proofs of identity and reads them back.
//
// Like password hashing, how a proof is signed is a thing that gets replaced, and
// naming the interface after the capability is what makes replacing it a matter of
// writing a second implementation instead of editing everything that signs.
type IAccessTokenProxy interface {
	// Issue signs a proof that this user is who they are, good until this moment.
	//
	// The moment arrives as an argument rather than being worked out here, and that
	// is deliberate: how long a session lasts is a rule this system chose, so it
	// belongs where the rules are. The other way round, changing "twenty-four hours"
	// would mean editing the part that knows about signatures.
	//
	// A system with no key to sign with refuses with ErrAccessTokenUnavailable
	// rather than handing back something unsigned, because an unsigned proof is one
	// anybody can write themselves.
	Issue(userID uint, expiresAt time.Time) (vo.AccessTokenVo, error)
	// UserIdentifiedBy reads a proof back and says whose it is. Missing, tampered
	// with, expired, or signed with another key are all ErrAuthenticationRequired:
	// four ways of not being a valid proof, and one thing for the holder to do.
	UserIdentifiedBy(accessToken string) (uint, error)
}

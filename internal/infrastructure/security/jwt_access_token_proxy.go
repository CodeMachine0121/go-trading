package security

import (
	"fmt"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/golang-jwt/jwt/v5"
)

// accessTokenSigningMethod is how a proof of identity is signed: one shared key,
// used both to sign and to check.
//
// A shared key is the right shape while the only thing that ever reads a proof is
// the same program that wrote it. The day something else has to read one — a second
// service, or anything running where the key cannot go — this becomes a key pair
// instead, and the way to do that is a second implementation of the interface, not
// an edit here.
var accessTokenSigningMethod = jwt.SigningMethodHS256

// acceptedSigningMethods is the one signing method a proof may claim to have been
// made with.
//
// Naming it is not belt-and-braces. A token says, in its own header, how it was
// signed — so a checker that believes the header can be handed a token claiming it
// needs no signature at all, and will agree. Pinning the method means the header is
// checked against what this system actually does rather than consulted about it.
var acceptedSigningMethods = []string{accessTokenSigningMethod.Alg()}

// JwtAccessTokenProxy issues proofs of identity and reads them back, as signed
// tokens carrying who they are for and when they stop counting.
//
// Nothing about an issued proof is kept. That is what makes signing in cost one
// write of nothing at all, and it is also why a proof cannot be taken back before it
// expires — the system has no list to strike it from. How long they last is the
// bound on that, and it is decided where the rules are, not here.
type JwtAccessTokenProxy struct {
	signingKey []byte
}

func NewJwtAccessTokenProxy(signingKey string) *JwtAccessTokenProxy {
	return &JwtAccessTokenProxy{signingKey: []byte(signingKey)}
}

// Issue signs a proof that this user is who they are, good until this moment.
//
// With no key, nothing is signed and nothing is handed back. The tempting
// alternative — sign with an empty key so that development is easier — produces
// proofs anybody can write for themselves, and a system that cannot tell a forged
// identity from a real one is worse than one that will not let anybody in at all.
func (jwtAccessTokenProxy *JwtAccessTokenProxy) Issue(
	userID uint, expiresAt time.Time,
) (vo.AccessTokenVo, error) {
	if len(jwtAccessTokenProxy.signingKey) == 0 {
		return vo.AccessTokenVo{}, fmt.Errorf(
			"%w: 尚未設定憑證簽章鑰匙", domains.ErrAccessTokenUnavailable)
	}

	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatUint(uint64(userID), 10),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	}

	signedToken, signError := jwt.NewWithClaims(accessTokenSigningMethod, claims).
		SignedString(jwtAccessTokenProxy.signingKey)
	if signError != nil {
		return vo.AccessTokenVo{}, fmt.Errorf(
			"%w: %w", domains.ErrAccessTokenUnavailable, signError)
	}

	return vo.AccessTokenVo{AccessToken: signedToken, ExpiresAt: expiresAt.UTC()}, nil
}

// UserIdentifiedBy reads a proof back and says whose it is.
//
// Every way of not being a valid proof arrives here as the same refusal: unreadable,
// signed with another key, altered after signing, expired, or carrying something
// where the identifier should be. They are one answer because they lead the holder
// to one action, and because telling them apart would describe the token to somebody
// who did not have a real one to begin with.
func (jwtAccessTokenProxy *JwtAccessTokenProxy) UserIdentifiedBy(accessToken string) (uint, error) {
	if len(jwtAccessTokenProxy.signingKey) == 0 {
		return 0, domains.ErrAuthenticationRequired
	}

	claims := jwt.RegisteredClaims{}

	// Expiry is checked here as part of reading the token, so a proof that is
	// perfectly signed but past its moment is refused by the same call that would
	// have accepted it an hour earlier.
	_, parseError := jwt.ParseWithClaims(
		accessToken,
		&claims,
		func(*jwt.Token) (any, error) { return jwtAccessTokenProxy.signingKey, nil },
		jwt.WithValidMethods(acceptedSigningMethods),
	)
	if parseError != nil {
		return 0, domains.ErrAuthenticationRequired
	}

	userID, parseIdentifierError := strconv.ParseUint(claims.Subject, 10, strconv.IntSize)
	if parseIdentifierError != nil || userID == 0 {
		return 0, domains.ErrAuthenticationRequired
	}

	return uint(userID), nil
}

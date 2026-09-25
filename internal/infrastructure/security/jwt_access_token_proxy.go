package security

import (
	"fmt"
	"strconv"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/golang-jwt/jwt/v5"
)

// accessTokenSigningMethod uses a shared HMAC key because only this service reads its tokens; switch to a key pair via a new implementation if that changes.
var accessTokenSigningMethod = jwt.SigningMethodHS256

// acceptedSigningMethods pins the algorithm so a token's header cannot claim a weaker one, such as none.
var acceptedSigningMethods = []string{accessTokenSigningMethod.Alg()}

// JwtAccessTokenProxy issues stateless signed tokens, so they cannot be revoked before expiry.
type JwtAccessTokenProxy struct {
	signingKey []byte
}

func NewJwtAccessTokenProxy(signingKey string) *JwtAccessTokenProxy {
	return &JwtAccessTokenProxy{signingKey: []byte(signingKey)}
}

// Issue refuses to sign without a key rather than producing forgeable tokens.
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

// UserIdentifiedBy returns one refusal for every invalid token so nothing about the token is revealed.
func (jwtAccessTokenProxy *JwtAccessTokenProxy) UserIdentifiedBy(accessToken string) (uint, error) {
	if len(jwtAccessTokenProxy.signingKey) == 0 {
		return 0, domains.ErrAuthenticationRequired
	}

	claims := jwt.RegisteredClaims{}

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

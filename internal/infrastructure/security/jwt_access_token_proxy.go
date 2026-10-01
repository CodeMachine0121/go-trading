package security

import (
	"fmt"
	"strconv"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/golang-jwt/jwt/v5"
)

// accessTokenSigningMethod uses a shared HMAC key because only this service reads its tokens; switch to a key pair via a new implementation if that changes.
var accessTokenSigningMethod = jwt.SigningMethodHS256

// acceptedSigningMethods pins the algorithm so a token's header cannot claim a weaker one, such as none.
var acceptedSigningMethods = []string{accessTokenSigningMethod.Alg()}

// accessTokenWireClaims carries the connector as client_id, the claim name RFC 9068 §2.2 gives it.
type accessTokenWireClaims struct {
	jwt.RegisteredClaims
	ClientIdentifier string `json:"client_id,omitempty"`
}

// JwtAccessTokenProxy issues stateless signed tokens, so they cannot be revoked before expiry.
type JwtAccessTokenProxy struct {
	signingKey []byte
}

func NewJwtAccessTokenProxy(signingKey string) *JwtAccessTokenProxy {
	return &JwtAccessTokenProxy{signingKey: []byte(signingKey)}
}

// Issue refuses to sign without a key rather than producing forgeable tokens.
func (jwtAccessTokenProxy *JwtAccessTokenProxy) Issue(
	claims vo.AccessTokenClaimsVo,
) (vo.AccessTokenVo, error) {
	if len(jwtAccessTokenProxy.signingKey) == 0 {
		return vo.AccessTokenVo{}, fmt.Errorf(
			"%w: 尚未設定憑證簽章鑰匙", domains.ErrAccessTokenUnavailable)
	}

	wireClaims := accessTokenWireClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(claims.UserID), 10),
			ExpiresAt: jwt.NewNumericDate(claims.ExpiresAt),
		},
		ClientIdentifier: claims.ConnectorClientIdentifier,
	}
	if claims.Audience != "" {
		wireClaims.Audience = jwt.ClaimStrings{claims.Audience}
	}

	signedToken, signError := jwt.NewWithClaims(accessTokenSigningMethod, wireClaims).
		SignedString(jwtAccessTokenProxy.signingKey)
	if signError != nil {
		return vo.AccessTokenVo{}, fmt.Errorf(
			"%w: %w", domains.ErrAccessTokenUnavailable, signError)
	}

	return vo.AccessTokenVo{AccessToken: signedToken, ExpiresAt: claims.ExpiresAt.UTC()}, nil
}

// ClaimsOf returns one refusal for every invalid token so nothing about the token is revealed; the audience is not checked here because the service accepts tokens with or without one.
func (jwtAccessTokenProxy *JwtAccessTokenProxy) ClaimsOf(accessToken string) (vo.AccessTokenClaimsVo, error) {
	if len(jwtAccessTokenProxy.signingKey) == 0 {
		return vo.AccessTokenClaimsVo{}, domains.ErrAuthenticationRequired
	}

	wireClaims := accessTokenWireClaims{}

	_, parseError := jwt.ParseWithClaims(
		accessToken,
		&wireClaims,
		func(*jwt.Token) (any, error) { return jwtAccessTokenProxy.signingKey, nil },
		jwt.WithValidMethods(acceptedSigningMethods),
		jwt.WithExpirationRequired(),
	)
	if parseError != nil {
		return vo.AccessTokenClaimsVo{}, domains.ErrAuthenticationRequired
	}

	userID, parseIdentifierError := strconv.ParseUint(wireClaims.Subject, 10, strconv.IntSize)
	if parseIdentifierError != nil || userID == 0 {
		return vo.AccessTokenClaimsVo{}, domains.ErrAuthenticationRequired
	}

	audience := ""
	if len(wireClaims.Audience) > 0 {
		audience = wireClaims.Audience[0]
	}

	return vo.AccessTokenClaimsVo{
		UserID:                    uint(userID),
		Audience:                  audience,
		ConnectorClientIdentifier: wireClaims.ClientIdentifier,
		ExpiresAt:                 wireClaims.ExpiresAt.UTC(),
	}, nil
}

func (jwtAccessTokenProxy *JwtAccessTokenProxy) UserIdentifiedBy(accessToken string) (uint, error) {
	claims, claimsError := jwtAccessTokenProxy.ClaimsOf(accessToken)
	if claimsError != nil {
		return 0, claimsError
	}

	return claims.UserID, nil
}

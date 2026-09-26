package security_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const signingKey = "test-signing-key"

var tokenExpiry = time.Date(2099, 9, 6, 8, 0, 0, 0, time.UTC)

func TestJwtAccessTokenProxyIssuesAProofItCanReadBack(t *testing.T) {
	accessTokenProxy := security.NewJwtAccessTokenProxy(signingKey)

	accessTokenVo, issueError := accessTokenProxy.Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: tokenExpiry})
	require.NoError(t, issueError)

	assert.NotEmpty(t, accessTokenVo.AccessToken)
	assert.Equal(t, tokenExpiry, accessTokenVo.ExpiresAt)

	userID, identifyError := accessTokenProxy.UserIdentifiedBy(accessTokenVo.AccessToken)

	require.NoError(t, identifyError)
	assert.Equal(t, uint(7), userID)
}

func TestJwtAccessTokenProxyRefusesEveryProofThatIsNotOne(t *testing.T) {
	accessTokenProxy := security.NewJwtAccessTokenProxy(signingKey)
	issuedToken, issueError := accessTokenProxy.Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: tokenExpiry})
	require.NoError(t, issueError)

	expiredToken, expiredError := accessTokenProxy.Issue(
		vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
	require.NoError(t, expiredError)

	otherKeyProxy := security.NewJwtAccessTokenProxy("another-signing-key")
	otherKeyToken, otherKeyError := otherKeyProxy.Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: tokenExpiry})
	require.NoError(t, otherKeyError)

	testCases := []struct {
		name        string
		accessToken string
	}{
		{name: "nothing at all", accessToken: ""},
		{name: "something that was never a token", accessToken: "not.a.token"},
		{
			name:        "altered after it was signed",
			accessToken: tamperedTail(issuedToken.AccessToken),
		},
		{
			name:        "perfectly signed, but its moment has passed",
			accessToken: expiredToken.AccessToken,
		},
		{name: "signed with somebody else's key", accessToken: otherKeyToken.AccessToken},
		{
			name:        "one that claims it needs no signature at all",
			accessToken: unsignedTokenFor("7"),
		},
		{
			// Same key, different method: accepting it would let the token choose how it is checked.
			name:        "one signed by a method this system does not use",
			accessToken: tokenSignedWithAnotherMethod(t, "7"),
		},
		{
			name:        "one carrying something that is not an identifier",
			accessToken: mustIssueForSubject(t, "not-a-number"),
		},
		{
			name:        "one carrying an identifier nobody can hold",
			accessToken: mustIssueForSubject(t, "0"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := accessTokenProxy.UserIdentifiedBy(testCase.accessToken)

			require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
		})
	}
}

func TestJwtAccessTokenProxyWithNoKeyRefusesToSignAnything(t *testing.T) {
	accessTokenProxy := security.NewJwtAccessTokenProxy("")

	_, err := accessTokenProxy.Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: tokenExpiry})

	require.ErrorIs(t, err, domains.ErrAccessTokenUnavailable,
		"沒有鑰匙時簽出一份沒簽章的憑證，等於任何人都能自己寫一份")
}

func TestJwtAccessTokenProxyWithNoKeyRecognisesNobody(t *testing.T) {
	signedElsewhere, issueError := security.NewJwtAccessTokenProxy(signingKey).Issue(vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: tokenExpiry})
	require.NoError(t, issueError)

	_, err := security.NewJwtAccessTokenProxy("").UserIdentifiedBy(signedElsewhere.AccessToken)

	require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
}

// tamperedTail breaks the signature without changing the token's shape.
func tamperedTail(accessToken string) string {
	lastCharacter := accessToken[len(accessToken)-1]
	replacement := byte('A')
	if lastCharacter == 'A' {
		replacement = 'B'
	}

	return accessToken[:len(accessToken)-1] + string(replacement)
}

// unsignedTokenFor builds an alg-none token.
func unsignedTokenFor(subject string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]string{"sub": subject})

	return header + "." + base64.RawURLEncoding.EncodeToString(claims) + "."
}

// mustIssueForSubject rebuilds a real token's payload with an arbitrary subject.
func mustIssueForSubject(t *testing.T, subject string) string {
	t.Helper()

	claims, marshalError := json.Marshal(map[string]any{
		"sub": subject,
		"exp": tokenExpiry.Unix(),
	})
	require.NoError(t, marshalError)

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString(claims)

	return header + "." + payload + "." + signatureOf(header+"."+payload)
}

// tokenSignedWithAnotherMethod signs with the same key but a different HMAC size.
func tokenSignedWithAnotherMethod(t *testing.T, subject string) string {
	t.Helper()

	signedToken, signError := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{
		Subject:   subject,
		ExpiresAt: jwt.NewNumericDate(tokenExpiry),
	}).SignedString([]byte(signingKey))
	require.NoError(t, signError)

	return signedToken
}

func signatureOf(signingInput string) string {
	return base64.RawURLEncoding.EncodeToString(hmacSha256([]byte(signingKey), signingInput))
}

func hmacSha256(key []byte, message string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(message))

	return mac.Sum(nil)
}

func TestJwtAccessTokenProxyWritesTheAudienceOnlyWhenThereIsOne(t *testing.T) {
	testCases := []struct {
		name     string
		audience string
	}{
		{name: "a connector token", audience: "https://mcp.example.com/mcp"},
		{name: "a web token", audience: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			accessTokenProxy := security.NewJwtAccessTokenProxy(signingKey)
			accessTokenVo, issueError := accessTokenProxy.Issue(
				vo.AccessTokenClaimsVo{UserID: 7, Audience: testCase.audience, ExpiresAt: tokenExpiry})
			require.NoError(t, issueError)

			claims, claimsError := accessTokenProxy.ClaimsOf(accessTokenVo.AccessToken)

			require.NoError(t, claimsError)
			assert.Equal(t, vo.AccessTokenClaimsVo{UserID: 7, Audience: testCase.audience, ExpiresAt: tokenExpiry}, claims)
			assert.Equal(t, testCase.audience != "", strings.Contains(decodedPayload(t, accessTokenVo.AccessToken), `"aud"`))
		})
	}
}

func TestJwtAccessTokenProxyAcceptsAConnectorTokenForTheServiceItself(t *testing.T) {
	accessTokenProxy := security.NewJwtAccessTokenProxy(signingKey)
	accessTokenVo, issueError := accessTokenProxy.Issue(
		vo.AccessTokenClaimsVo{UserID: 7, Audience: "https://mcp.example.com/mcp", ExpiresAt: tokenExpiry})
	require.NoError(t, issueError)

	userID, identifyError := accessTokenProxy.UserIdentifiedBy(accessTokenVo.AccessToken)

	require.NoError(t, identifyError)
	assert.Equal(t, uint(7), userID)
}

func TestJwtAccessTokenProxyReadsNoClaimsFromAnInvalidToken(t *testing.T) {
	expired, issueError := security.NewJwtAccessTokenProxy(signingKey).Issue(
		vo.AccessTokenClaimsVo{UserID: 7, ExpiresAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)})
	require.NoError(t, issueError)
	noSubject, signError := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(tokenExpiry),
	}).SignedString([]byte(signingKey))
	require.NoError(t, signError)
	noExpiry, noExpirySignError := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject: "7",
	}).SignedString([]byte(signingKey))
	require.NoError(t, noExpirySignError)

	testCases := []struct {
		name             string
		accessTokenProxy *security.JwtAccessTokenProxy
		accessToken      string
	}{
		{name: "expired", accessTokenProxy: security.NewJwtAccessTokenProxy(signingKey), accessToken: expired.AccessToken},
		{name: "no subject", accessTokenProxy: security.NewJwtAccessTokenProxy(signingKey), accessToken: noSubject},
		{name: "no expiry", accessTokenProxy: security.NewJwtAccessTokenProxy(signingKey), accessToken: noExpiry},
		{name: "no key configured", accessTokenProxy: security.NewJwtAccessTokenProxy(""), accessToken: expired.AccessToken},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := testCase.accessTokenProxy.ClaimsOf(testCase.accessToken)

			assert.ErrorIs(t, err, domains.ErrAuthenticationRequired)
		})
	}
}

func decodedPayload(t *testing.T, accessToken string) string {
	t.Helper()
	segments := strings.Split(accessToken, ".")
	require.Len(t, segments, 3)
	payload, decodeError := base64.RawURLEncoding.DecodeString(segments[1])
	require.NoError(t, decodeError)

	return string(payload)
}

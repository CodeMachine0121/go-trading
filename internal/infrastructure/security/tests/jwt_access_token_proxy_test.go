package security_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const signingKey = "test-signing-key"

// tokenExpiry is far enough ahead that these tests never race the clock.
var tokenExpiry = time.Date(2099, 9, 6, 8, 0, 0, 0, time.UTC)

func TestJwtAccessTokenProxyIssuesAProofItCanReadBack(t *testing.T) {
	accessTokenProxy := security.NewJwtAccessTokenProxy(signingKey)

	accessTokenVo, issueError := accessTokenProxy.Issue(7, tokenExpiry)
	require.NoError(t, issueError)

	assert.NotEmpty(t, accessTokenVo.AccessToken)
	assert.Equal(t, tokenExpiry, accessTokenVo.ExpiresAt)

	userID, identifyError := accessTokenProxy.UserIdentifiedBy(accessTokenVo.AccessToken)

	require.NoError(t, identifyError)
	assert.Equal(t, uint(7), userID)
}

func TestJwtAccessTokenProxyRefusesEveryProofThatIsNotOne(t *testing.T) {
	accessTokenProxy := security.NewJwtAccessTokenProxy(signingKey)
	issuedToken, issueError := accessTokenProxy.Issue(7, tokenExpiry)
	require.NoError(t, issueError)

	expiredToken, expiredError := accessTokenProxy.Issue(
		7, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, expiredError)

	otherKeyProxy := security.NewJwtAccessTokenProxy("another-signing-key")
	otherKeyToken, otherKeyError := otherKeyProxy.Issue(7, tokenExpiry)
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
			// Signed with the very same key, correctly, by a method this system does
			// not use. Accepting it would mean the token gets to nominate how it is
			// checked — and a token is written by whoever presents it.
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

	_, err := accessTokenProxy.Issue(7, tokenExpiry)

	require.ErrorIs(t, err, domains.ErrAccessTokenUnavailable,
		"沒有鑰匙時簽出一份沒簽章的憑證，等於任何人都能自己寫一份")
}

func TestJwtAccessTokenProxyWithNoKeyRecognisesNobody(t *testing.T) {
	signedElsewhere, issueError := security.NewJwtAccessTokenProxy(signingKey).Issue(7, tokenExpiry)
	require.NoError(t, issueError)

	_, err := security.NewJwtAccessTokenProxy("").UserIdentifiedBy(signedElsewhere.AccessToken)

	require.ErrorIs(t, err, domains.ErrAuthenticationRequired)
}

// tamperedTail changes the last character of a token, which is enough to break the
// signature without changing the shape of the thing.
func tamperedTail(accessToken string) string {
	lastCharacter := accessToken[len(accessToken)-1]
	replacement := byte('A')
	if lastCharacter == 'A' {
		replacement = 'B'
	}

	return accessToken[:len(accessToken)-1] + string(replacement)
}

// unsignedTokenFor builds a token that names no signing method, which is the oldest
// way of walking in through a checker that believes what a token says about itself.
func unsignedTokenFor(subject string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	claims, _ := json.Marshal(map[string]string{"sub": subject})

	return header + "." + base64.RawURLEncoding.EncodeToString(claims) + "."
}

// mustIssueForSubject signs a token whose subject is whatever is handed in, by
// rebuilding the middle of a real one. It is how a proof carrying nonsense where the
// identifier belongs is produced without an interface for producing nonsense.
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

// tokenSignedWithAnotherMethod signs a genuine token with the same key but a
// different HMAC size, which is the shape of every "let the token choose the
// algorithm" mistake.
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

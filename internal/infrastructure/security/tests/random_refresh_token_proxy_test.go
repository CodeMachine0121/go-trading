package security_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// digestLength is SHA-256 written in hexadecimal. It is fixed whatever went in,
// which is what lets the column that stores it be a fixed width.
const digestLength = 64

// minimumRefreshTokenLength is what the standard library's token generator produces
// today, carrying at least 128 bits of randomness. The requirement is "cannot be
// guessed", not any particular count — so this is a floor, not an equality.
const minimumRefreshTokenLength = 26

func TestRandomRefreshTokenProxyMintNeverHandsBackTheDigestAsTheValue(t *testing.T) {
	refreshTokenProxy := security.NewRandomRefreshTokenProxy()

	refreshToken, err := refreshTokenProxy.Mint()

	require.NoError(t, err)
	assert.NotEmpty(t, refreshToken.Value)
	assert.NotEqual(t, refreshToken.Value, refreshToken.Digest,
		"兩者交換位置的話，交出去的是開不了門的東西，留下來的是還能用的東西")
	assert.GreaterOrEqual(t, len(refreshToken.Value), minimumRefreshTokenLength)
	assert.Len(t, refreshToken.Digest, digestLength)
}

func TestRandomRefreshTokenProxyMintNeverRepeatsItself(t *testing.T) {
	refreshTokenProxy := security.NewRandomRefreshTokenProxy()

	first, firstError := refreshTokenProxy.Mint()
	second, secondError := refreshTokenProxy.Mint()

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.NotEqual(t, first.Value, second.Value, "猜不到是這件事唯一的價值")
	assert.NotEqual(t, first.Digest, second.Digest)
}

func TestRandomRefreshTokenProxyDigestOfAgreesWithWhatWasMinted(t *testing.T) {
	// If minting and looking up ever derived differently, every holder would be
	// locked out at the exact moment they tried to renew.
	refreshTokenProxy := security.NewRandomRefreshTokenProxy()

	refreshToken, err := refreshTokenProxy.Mint()

	require.NoError(t, err)
	assert.Equal(t, refreshToken.Digest, refreshTokenProxy.DigestOf(refreshToken.Value))
}

func TestRandomRefreshTokenProxyDigestOfIsTheSameEveryTime(t *testing.T) {
	// The opposite of a password proof, and deliberately so: a session has no
	// address to be looked up by — the proof is the address — so the derivation has
	// to be repeatable or nothing could be found at all.
	refreshTokenProxy := security.NewRandomRefreshTokenProxy()

	assert.Equal(t, refreshTokenProxy.DigestOf("a-refresh-token"),
		refreshTokenProxy.DigestOf("a-refresh-token"))
}

func TestRandomRefreshTokenProxyDigestOf(t *testing.T) {
	refreshTokenProxy := security.NewRandomRefreshTokenProxy()

	testCases := []struct {
		name         string
		refreshToken string
	}{
		{name: "an ordinary proof", refreshToken: "a-refresh-token"},
		{name: "nothing at all", refreshToken: ""},
		{name: "something very long", refreshToken: string(make([]byte, 4096))},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Len(t, refreshTokenProxy.DigestOf(testCase.refreshToken), digestLength)
		})
	}
}

func TestRandomRefreshTokenProxyDigestOfTellsDifferentProofsApart(t *testing.T) {
	refreshTokenProxy := security.NewRandomRefreshTokenProxy()

	assert.NotEqual(t, refreshTokenProxy.DigestOf("one"), refreshTokenProxy.DigestOf("two"))
}

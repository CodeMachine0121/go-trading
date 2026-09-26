package security_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimumOpaqueIdentifierLength is a floor (at least 128 bits of randomness), not an exact length.
const minimumOpaqueIdentifierLength = 26

func TestRandomOpaqueIdentifierProxyMint(t *testing.T) {
	opaqueIdentifierProxy := security.NewRandomOpaqueIdentifierProxy()

	first, firstError := opaqueIdentifierProxy.Mint()
	second, secondError := opaqueIdentifierProxy.Mint()

	require.NoError(t, firstError)
	require.NoError(t, secondError)
	assert.GreaterOrEqual(t, len(first.Value), minimumOpaqueIdentifierLength)
	assert.Len(t, first.Digest, digestLength)
	assert.NotEqual(t, first.Value, first.Digest)
	assert.NotEqual(t, first.Value, second.Value, "猜不到是這件事唯一的價值")
}

func TestRandomOpaqueIdentifierProxyDigestOf(t *testing.T) {
	opaqueIdentifierProxy := security.NewRandomOpaqueIdentifierProxy()
	minted, mintError := opaqueIdentifierProxy.Mint()
	require.NoError(t, mintError)

	testCases := []struct {
		name           string
		identifier     string
		expectedDigest string
	}{
		{name: "agrees with what was minted", identifier: minted.Value, expectedDigest: minted.Digest},
		{name: "is the same every time", identifier: "an-identifier",
			expectedDigest: opaqueIdentifierProxy.DigestOf("an-identifier")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.Equal(t, testCase.expectedDigest, opaqueIdentifierProxy.DigestOf(testCase.identifier))
		})
	}

	assert.NotEqual(t, opaqueIdentifierProxy.DigestOf("one"), opaqueIdentifierProxy.DigestOf("two"))
}

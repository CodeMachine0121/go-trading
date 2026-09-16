package security_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aSealKey is thirty-two bytes written as base64, the only shape a usable key has.
var aSealKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))

func TestAesSecretSealProxyGivesBackWhatItWasGiven(t *testing.T) {
	secretSealProxy := security.NewAesSecretSealProxy(aSealKey)

	sealed, sealError := secretSealProxy.Seal("123456:AAHqwertyuiop1234")
	require.NoError(t, sealError)

	opened, unsealError := secretSealProxy.Unseal(sealed)

	require.NoError(t, unsealError)
	assert.Equal(t, "123456:AAHqwertyuiop1234", opened)
}

// A bot token is not a password: the system needs it whole, every time it sends. So
// it is locked rather than hashed — and what is stored must not read as the token.
func TestAesSecretSealProxyStoresSomethingThatIsNotTheSecret(t *testing.T) {
	secretSealProxy := security.NewAesSecretSealProxy(aSealKey)

	sealed, sealError := secretSealProxy.Seal("123456:AAHqwertyuiop1234")

	require.NoError(t, sealError)
	assert.NotContains(t, sealed, "AAHqwertyuiop")
	assert.NotContains(t, sealed, "123456")
}

// Without this, the table would show at a glance which people pasted the same
// token.
func TestAesSecretSealProxySealsTheSameSecretDifferentlyEveryTime(t *testing.T) {
	secretSealProxy := security.NewAesSecretSealProxy(aSealKey)

	first, firstError := secretSealProxy.Seal("the-same-token")
	require.NoError(t, firstError)
	second, secondError := secretSealProxy.Seal("the-same-token")
	require.NoError(t, secondError)

	assert.NotEqual(t, first, second)

	firstOpened, firstOpenError := secretSealProxy.Unseal(first)
	require.NoError(t, firstOpenError)
	secondOpened, secondOpenError := secretSealProxy.Unseal(second)
	require.NoError(t, secondOpenError)
	assert.Equal(t, firstOpened, secondOpened)
}

// Refusing is the whole feature. Storing the token in the open would work
// perfectly, right up until somebody read the table — and nothing would look
// different in the meantime.
func TestAesSecretSealProxyRefusesEverythingWhenItHasNoKey(t *testing.T) {
	testCases := []struct {
		name string
		key  string
	}{
		{name: "no key configured at all", key: ""},
		{name: "a key that is not base64", key: "not base64 at all!!"},
		{
			name: "a key of the wrong length",
			key:  base64.StdEncoding.EncodeToString([]byte("too short")),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			secretSealProxy := security.NewAesSecretSealProxy(testCase.key)

			sealed, sealError := secretSealProxy.Seal("123456:AAHqwertyuiop1234")
			require.ErrorIs(t, sealError, domains.ErrSecretSealUnavailable)
			assert.Empty(t, sealed, "拒絕保存的那一次不得吐出任何可以被存下去的東西")

			_, unsealError := secretSealProxy.Unseal("anything")
			require.ErrorIs(t, unsealError, domains.ErrSecretSealUnavailable)
		})
	}
}

// Anything that does not open is an error rather than an empty string. An empty
// token would be spent as though it were real, and the destination's refusal would
// reach the person as "your token is wrong" when the system is the thing that lost
// it.
func TestAesSecretSealProxyRefusesStoredContentItCannotOpen(t *testing.T) {
	secretSealProxy := security.NewAesSecretSealProxy(aSealKey)
	sealed, sealError := secretSealProxy.Seal("123456:AAHqwertyuiop1234")
	require.NoError(t, sealError)

	otherKey := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("j", 32)))

	testCases := []struct {
		name   string
		proxy  *security.AesSecretSealProxy
		sealed string
	}{
		{name: "content that is not base64", proxy: secretSealProxy, sealed: "not base64!!"},
		{
			name:   "content shorter than its own random material",
			proxy:  secretSealProxy,
			sealed: base64.StdEncoding.EncodeToString([]byte("tiny")),
		},
		{
			name:   "content altered after it was stored",
			proxy:  secretSealProxy,
			sealed: sealed[:len(sealed)-6] + "AAAAAA",
		},
		{
			name:   "content sealed with a different key",
			proxy:  security.NewAesSecretSealProxy(otherKey),
			sealed: sealed,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			opened, unsealError := testCase.proxy.Unseal(testCase.sealed)

			require.Error(t, unsealError)
			assert.Empty(t, opened)
		})
	}
}

package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// secretSealKeyLength is how long the key must be, in bytes, once decoded. It is the
// size AES uses for its strongest variant, and it is fixed rather than accepted at
// whatever length arrives: a key that is merely "some bytes" invites a short one,
// and a short one fails at the moment somebody first tries to store a token rather
// than at startup.
const secretSealKeyLength = 32

// AesSecretSealProxy locks secrets away with AES-GCM and opens them again.
//
// GCM is chosen over a plain cipher mode because it authenticates as well as hides.
// Without that, a stored token could be altered by whoever could write to the table
// and would come back out as different bytes rather than as a failure — and those
// bytes would then be sent to Telegram as somebody's credentials.
//
// Every seal draws a fresh nonce, so the same token sealed twice looks like two
// unrelated values. That is not decoration: without it, the table would show at a
// glance which people had pasted the same token.
type AesSecretSealProxy struct {
	// aeadCipher is nil when no usable key was configured. It is a single nil
	// check rather than a flag beside it, because a flag and a cipher can
	// disagree and there is no safe direction for that disagreement.
	aeadCipher cipher.AEAD
}

// NewAesSecretSealProxy reads the configured key and builds the cipher once.
//
// A missing or unusable key is not an error here. Refusing to start would take down
// candles, strategy scripts and sign-in over a feature nobody may be using; instead this
// proxy exists in a state where both of its methods refuse, and only the paths that
// actually need a key find out. What must never happen — storing a token unlocked —
// cannot, because there is no code path that does it.
func NewAesSecretSealProxy(base64Key string) *AesSecretSealProxy {
	key, decodeError := base64.StdEncoding.DecodeString(base64Key)
	if decodeError != nil || len(key) != secretSealKeyLength {
		return &AesSecretSealProxy{}
	}

	block, blockError := aes.NewCipher(key)
	if blockError != nil {
		return &AesSecretSealProxy{}
	}

	aeadCipher, aeadError := cipher.NewGCM(block)
	if aeadError != nil {
		return &AesSecretSealProxy{}
	}

	return &AesSecretSealProxy{aeadCipher: aeadCipher}
}

// Seal locks a secret into the single string that gets stored: the nonce followed
// by the sealed bytes, encoded as text.
//
// The nonce travels with the secret rather than being stored in a column of its own,
// because the two are useless apart and a schema that can hold one without the other
// is a schema in which they can be separated.
func (aesSecretSealProxy *AesSecretSealProxy) Seal(plaintext string) (string, error) {
	if aesSecretSealProxy.aeadCipher == nil {
		return "", domains.ErrSecretSealUnavailable
	}

	nonce := make([]byte, aesSecretSealProxy.aeadCipher.NonceSize())
	if _, randomError := rand.Read(nonce); randomError != nil {
		return "", fmt.Errorf("seal secret: %w", randomError)
	}

	sealed := aesSecretSealProxy.aeadCipher.Seal(nonce, nonce, []byte(plaintext), nil)

	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Unseal recovers a secret from its stored form.
//
// Anything that does not open — truncated, altered, sealed with a different key — is
// an error rather than an empty string. An empty token would be spent as though it
// were real, and the destination's refusal would reach the person as "your token is
// wrong" when the token is fine and the system lost it.
func (aesSecretSealProxy *AesSecretSealProxy) Unseal(sealed string) (string, error) {
	if aesSecretSealProxy.aeadCipher == nil {
		return "", domains.ErrSecretSealUnavailable
	}

	sealedBytes, decodeError := base64.StdEncoding.DecodeString(sealed)
	if decodeError != nil {
		return "", fmt.Errorf("unseal secret: %w", decodeError)
	}

	nonceSize := aesSecretSealProxy.aeadCipher.NonceSize()
	if len(sealedBytes) < nonceSize {
		return "", fmt.Errorf("unseal secret: 保存的內容比它的隨機料還短")
	}

	plaintext, openError := aesSecretSealProxy.aeadCipher.Open(
		nil, sealedBytes[:nonceSize], sealedBytes[nonceSize:], nil)
	if openError != nil {
		return "", fmt.Errorf("unseal secret: %w", openError)
	}

	return string(plaintext), nil
}

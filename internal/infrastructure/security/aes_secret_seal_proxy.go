package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// secretSealKeyLength is the required decoded key length in bytes (AES-256), enforced so a short key fails at startup.
const secretSealKeyLength = 32

// AesSecretSealProxy uses AES-GCM so tampered ciphertext fails instead of decrypting to different bytes, with a fresh nonce per seal so equal secrets look unrelated.
type AesSecretSealProxy struct {
	// aeadCipher is nil when no usable key was configured.
	aeadCipher cipher.AEAD
}

// NewAesSecretSealProxy tolerates a missing or invalid key so unrelated features still start; both methods then refuse.
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

// Seal returns the nonce followed by the ciphertext as one encoded string, so the two cannot be stored apart.
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

// Unseal fails, rather than returning an empty string, on anything that does not open.
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

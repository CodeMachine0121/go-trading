package _interface

//go:generate go tool mockgen -source=i_secret_seal_proxy.go -destination=mocks/mock_i_secret_seal_proxy.go -package=mocks

// ISecretSealProxy reversibly encrypts secrets such as bot tokens, unlike IPasswordProofProxy's one-way hashing.
// Without a key both methods return ErrSecretSealUnavailable; there is deliberately no fallback that stores a secret unencrypted.
type ISecretSealProxy interface {
	// Seal is randomized, so the same secret yields different results.
	Seal(plaintext string) (string, error)
	// Unseal errors on unreadable input rather than returning an empty token that would be sent as if real.
	Unseal(sealed string) (string, error)
}

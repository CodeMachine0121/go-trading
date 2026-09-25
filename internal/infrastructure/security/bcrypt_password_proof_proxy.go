package security

import (
	"cmp"
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// passwordProofCost 12 keeps one derivation in the low hundreds of milliseconds, targeting sign-in under a second; raise it as hardware gets faster.
const passwordProofCost = 12

// fallbackDecoyProof is a proof of a discarded random password, used only if randomness is unavailable at startup.
const fallbackDecoyProof = "$2a$12$XnhfeGHwjLbM/cah350NkOeZnpiIZUnm8UF4w3HoxjbuZbxdkrzl6"

type BcryptPasswordProofProxy struct {
	decoyProof string
}

// NewBcryptPasswordProofProxy derives the decoy proof once at startup rather than per failed sign-in.
func NewBcryptPasswordProofProxy() *BcryptPasswordProofProxy {
	return &BcryptPasswordProofProxy{decoyProof: newDecoyProof()}
}

// Prove rejects passwords over bcrypt's 72-byte limit instead of silently proving only a prefix.
func (bcryptPasswordProofProxy *BcryptPasswordProofProxy) Prove(password string) (string, error) {
	passwordProof, deriveError := bcrypt.GenerateFromPassword([]byte(password), passwordProofCost)
	if deriveError != nil {
		return "", fmt.Errorf("derive password proof: %w", deriveError)
	}

	return string(passwordProof), nil
}

// Matches returns false for an unreadable proof, and checks an empty proof against the decoy so a missing account costs the same time as a wrong password.
func (bcryptPasswordProofProxy *BcryptPasswordProofProxy) Matches(
	password string, passwordProof string,
) bool {
	return bcrypt.CompareHashAndPassword(
		[]byte(cmp.Or(passwordProof, bcryptPasswordProofProxy.decoyProof)),
		[]byte(password),
	) == nil
}

// newDecoyProof ignores the bcrypt error because its inputs are constants that cannot fail, but falls back to fallbackDecoyProof if the result is empty.
func newDecoyProof() string {
	decoyProof, _ := bcrypt.GenerateFromPassword([]byte(rand.Text()), passwordProofCost)

	return cmp.Or(string(decoyProof), fallbackDecoyProof)
}

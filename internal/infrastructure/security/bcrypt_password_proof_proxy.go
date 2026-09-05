package security

import (
	"cmp"
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// passwordProofCost is how much work deriving one proof takes. Every step up
// doubles it.
//
// A password hashing scheme is supposed to be slow, which reads like a defect until
// you count from the other side: whoever steals the table guesses billions of
// passwords, and the only thing standing between them and the weak ones is what each
// guess costs. Anything fast enough to be free for us is free for them too.
//
// Twelve puts one derivation in the low hundreds of milliseconds on the hardware
// this runs on — unnoticeable inside a sign-in that already crosses a network, and
// roughly four thousand times the cost of a scheme that was never meant for
// passwords. The requirement it is chosen against is "a sign-in stays under a
// second"; raise it when hardware makes that too easy.
const passwordProofCost = 12

// fallbackDecoyProof is a proof of a password that was generated at random and
// thrown away, kept for the one case where the machine cannot produce randomness at
// startup.
//
// It is safe to have in the source precisely because nobody — including whoever
// wrote this line — knows the password it came from. What it costs to check is what
// matters, and that is the same as any other proof at the cost it was made with.
const fallbackDecoyProof = "$2a$12$XnhfeGHwjLbM/cah350NkOeZnpiIZUnm8UF4w3HoxjbuZbxdkrzl6"

// BcryptPasswordProofProxy derives storable proofs of passwords, and checks
// passwords against them, using bcrypt.
//
// bcrypt salts every derivation itself, which is why the same password proved twice
// gives two different proofs and why the salt is not something this code has to
// remember: it travels inside the proof.
type BcryptPasswordProofProxy struct {
	decoyProof string
}

// NewBcryptPasswordProofProxy derives the decoy proof once, at startup, rather than
// per failed sign-in. Deriving it each time would spend the cost twice on the one
// path that is already the slowest.
func NewBcryptPasswordProofProxy() *BcryptPasswordProofProxy {
	return &BcryptPasswordProofProxy{decoyProof: newDecoyProof()}
}

// Prove derives the storable proof of a password.
//
// A password too long for the scheme to read whole comes back as a failure. bcrypt
// stops at seventy-two bytes, so accepting a longer one would store a proof of its
// beginning while its owner believes the whole thing guards their account. The rules
// refuse that length before it ever reaches here; this is the second lock on the
// same door, for whoever wires up a third caller one day.
func (bcryptPasswordProofProxy *BcryptPasswordProofProxy) Prove(password string) (string, error) {
	passwordProof, deriveError := bcrypt.GenerateFromPassword([]byte(password), passwordProofCost)
	if deriveError != nil {
		return "", fmt.Errorf("derive password proof: %w", deriveError)
	}

	return string(passwordProof), nil
}

// Matches says whether this password is the one the proof was derived from.
//
// A proof it cannot read at all comes back as "no", not as an error: it is not
// something the person signing in did, and there is nothing they could do about it.
// What they need to be told is the same thing a wrong password tells them.
func (bcryptPasswordProofProxy *BcryptPasswordProofProxy) Matches(
	password string, passwordProof string,
) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordProof), []byte(password)) == nil
}

// DecoyProof is a proof no password matches, for the case where there is no account
// to check against. See IPasswordProofProxy for why checking against it beats
// returning early.
func (bcryptPasswordProofProxy *BcryptPasswordProofProxy) DecoyProof() string {
	return bcryptPasswordProofProxy.decoyProof
}

// newDecoyProof derives a proof of a password generated at random and immediately
// forgotten, so that checking against it costs exactly what checking against a real
// account costs.
//
// The failure is not branched on, and that is a decision rather than an oversight.
// Random text is documented never to fail, and the derivation refuses exactly two
// things — a cost outside its range and a password over seventy-two bytes — both of
// which are constants here. A branch on it would be one nothing can reach and no
// test can enter.
//
// What is not ignored is the consequence. An empty proof would be refused instantly,
// which is the very fast answer this whole idea exists to avoid, so an empty result
// falls back to a proof made the same way at the same cost on somebody else's
// machine. That is a fact about the value, not a guess about the error.
func newDecoyProof() string {
	decoyProof, _ := bcrypt.GenerateFromPassword([]byte(rand.Text()), passwordProofCost)

	return cmp.Or(string(decoyProof), fallbackDecoyProof)
}

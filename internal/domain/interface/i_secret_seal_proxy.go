package _interface

//go:generate go tool mockgen -source=i_secret_seal_proxy.go -destination=mocks/mock_i_secret_seal_proxy.go -package=mocks

// ISecretSealProxy locks away a secret that has to be read back, and opens it again.
//
// It is not IPasswordProofProxy and must never be mistaken for it. A password proof
// cannot be reversed, and that is fine because nobody ever needs the password again
// — only whether a guess matches. A bot token is the opposite: it is worthless
// unless it can be produced whole, every time a message goes out. So it cannot be
// hashed, only locked, and what makes locking worth anything is that the key is not
// kept beside the lock.
//
// It is named for the capability rather than for the algorithm, for the same reason
// password hashing is: every scheme in use has a predecessor that was also once the
// right answer.
//
// With no key, both methods refuse with ErrSecretSealUnavailable. There is
// deliberately no third method asking whether a key is present, and deliberately no
// path that stores a secret unlocked when one is missing. Such a path would make
// setting up delivery appear to work and messages actually send, with nothing
// visibly different until the day somebody read the table.
type ISecretSealProxy interface {
	// Seal locks a secret into the form that may be stored. The same secret sealed
	// twice yields two different results, so what is stored never reveals that two
	// people pasted the same token.
	Seal(plaintext string) (string, error)
	// Unseal recovers a secret from its stored form. A stored form this cannot
	// read is an error and not an empty string: an empty token would be spent as
	// if it were real, and the destination's refusal would be reported to the
	// person as though their token were wrong.
	Unseal(sealed string) (string, error)
}

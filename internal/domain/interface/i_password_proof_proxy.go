package _interface

//go:generate go tool mockgen -source=i_password_proof_proxy.go -destination=mocks/mock_i_password_proof_proxy.go -package=mocks

// IPasswordProofProxy turns a password into something that can be stored in its
// place, and says whether a password matches one of those.
//
// It is named for the capability and not for the algorithm on purpose. Password
// hashing is a thing that gets replaced — every scheme in use has a predecessor that
// was also once the right answer — and the whole point of putting it behind an
// interface is that the day it is replaced, nothing above this line changes. The
// rules that are actually this system's own (a password is at least eight
// characters; at most seventy-two bytes) live in the domain, where they can be read
// and tested without any of this.
type IPasswordProofProxy interface {
	// Prove derives the storable proof of a password. The same password proved twice
	// yields two different proofs, so what is stored never reveals that two people
	// chose the same one.
	Prove(password string) (string, error)
	// Matches says whether this password is the one the proof was derived from. A
	// proof it cannot even read is not a match — it is not an error either, because
	// there is nothing the person signing in could do about it.
	Matches(password string, passwordProof string) bool
	// DecoyProof is a proof no password matches, for the case where there is no
	// account to check against.
	//
	// It exists because of what the alternative sounds like. Returning straight away
	// when nobody holds an address makes that answer arrive measurably sooner than a
	// wrong password does — and "this one came back faster" is the same information
	// as "no such account", handed over without anybody deciding to. Checking
	// against a decoy spends the same effort, so the two failures cost the same.
	//
	// It belongs here rather than in the domain because only the side that knows the
	// algorithm can produce something the algorithm will actually work through.
	DecoyProof() string
}

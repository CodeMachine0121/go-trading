package _interface

//go:generate go tool mockgen -source=i_password_proof_proxy.go -destination=mocks/mock_i_password_proof_proxy.go -package=mocks

// IPasswordProofProxy derives storable password proofs and checks passwords against them; length rules live in the domain.
type IPasswordProofProxy interface {
	// Prove is salted, so the same password yields different proofs.
	Prove(password string) (string, error)
	// Matches treats an unreadable proof as a non-match, and an empty proof costs as much as a real check so response timing never reveals whether an account exists.
	Matches(password string, passwordProof string) bool
}

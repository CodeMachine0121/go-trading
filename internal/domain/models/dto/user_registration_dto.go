package dto

// UserRegistrationDto carries the plaintext password only as far as the domain service that
// hashes it.
type UserRegistrationDto struct {
	Email    string
	Password string
}

package dto

// SignInDto is kept separate from UserRegistrationDto because the two are validated by
// different rules.
type SignInDto struct {
	Email    string
	Password string
}

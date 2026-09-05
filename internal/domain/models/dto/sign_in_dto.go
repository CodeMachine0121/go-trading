package dto

// SignInDto is what the application hands the domain to sign somebody in.
//
// It is deliberately a separate shape from UserRegistrationDto even though it holds
// the same two fields: the rules the two are judged by are not the same, and one
// shape shared between them would be an invitation to share the rules too. What
// makes a password acceptable to register with says nothing about whether it is the
// right password now.
type SignInDto struct {
	Email    string
	Password string
}

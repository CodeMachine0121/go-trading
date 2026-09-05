package dto

// UserRegistrationDto is what the application hands the domain to create a user:
// the email address that will be the account, and the password as typed.
//
// The password travels as it was typed because this is the one moment the system is
// allowed to see it — the proof cannot be derived from anything else. It goes no
// further than the domain service that turns it into a proof.
type UserRegistrationDto struct {
	Email    string
	Password string
}

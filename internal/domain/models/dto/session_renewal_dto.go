package dto

// SessionRenewalDto is what the application hands the domain to renew or to end a
// session: the renewal proof, and nothing else.
//
// Nothing else is the interesting part. Renewing deliberately does not want the
// access token — the whole reason renewal exists is for when that one has expired —
// and it does not want the password either, because not having to ask for the
// password again is the point.
type SessionRenewalDto struct {
	RefreshToken string
}

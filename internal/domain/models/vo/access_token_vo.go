package vo

import "time"

// AccessTokenVo is one issued proof of identity, as the signing side hands it back:
// the token and the moment it expires. Immutable, no behavior.
//
// It exists so that whoever issues a token cannot hand back only the token and
// leave the expiry to be guessed at — the two are one answer, and separating them
// is how "when does this stop working" becomes nobody's job to say.
//
// It has no conversion of its own: what leaves the domain is never one token, it is
// always the pair (see SessionTokensVo).
type AccessTokenVo struct {
	AccessToken string
	ExpiresAt   time.Time
}

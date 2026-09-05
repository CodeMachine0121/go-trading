package dto

import "time"

// SessionTokensDto is what opening or renewing a session hands back: the proof every
// request carries, the proof that renews it, and the moment each of them stops
// counting.
//
// It replaces a shape that carried one token. The name changed rather than the
// fields being added to it, because it no longer holds "an access token" — it holds
// a pair, and a name that says otherwise makes every reader misunderstand it once
// before they read the fields.
//
// Both expiries travel alongside their tokens rather than being left to be read out
// of them. The renewal proof has nothing inside it to read — it is random — and the
// access token's innards are deliberately the system's business alone.
type SessionTokensDto struct {
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
	// RefreshToken is handed out here and nowhere else. What is stored is derived
	// from it and cannot be turned back into it, so this response is the only chance
	// its holder gets.
	RefreshToken          string    `json:"refreshToken"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

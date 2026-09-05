package entities

import "time"

// Session is one sign-in on one device: who it belongs to, the proof that keeps it
// alive, and when it stops. It is a plain data model — fields and persistence
// mapping only, no business rules.
//
// It exists because the other proof — the one every request carries — deliberately
// is not stored. Not storing it is what makes checking it free; the price is that it
// cannot be taken back. This row is where that price is paid instead: it is checked
// rarely (a renewal, a sign-out), so it can afford to be state.
type Session struct {
	ID uint `gorm:"primaryKey"`
	// UserID cascades on delete, which is the whole of "a user who is gone has no
	// sessions". Said here rather than in code somewhere that has to remember to
	// tidy up, because a row nobody remembers to delete is a row that stays forever.
	UserID uint `gorm:"not null;index"`
	// ChainID is shared by every session this one sign-in has been renewed into.
	//
	// A chain is one device's one sign-in. It exists so that ending it — whether
	// because somebody signed out or because a used proof turned up again — is one
	// query, instead of walking a linked list of renewals one hop at a time.
	ChainID string `gorm:"size:64;not null;index"`
	// RefreshTokenDigest is what is kept instead of the renewal proof itself, and it
	// is also what this row is found by. Being findable is exactly why it is derived
	// without salt, unlike a password proof — see RandomRefreshTokenProxy for why
	// that is safe here and would not be for a password.
	RefreshTokenDigest string    `gorm:"size:64;not null;uniqueIndex:idx_sessions_refresh_token_digest"`
	ExpiresAt          time.Time `gorm:"type:timestamptz;not null"`
	// RevokedAt is a moment rather than a flag because knowing when costs nothing
	// extra and without it no later investigation has anywhere to begin. Nil means the
	// session is still good.
	RevokedAt *time.Time `gorm:"type:timestamptz"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null"`
}

// TableName pins the table to Sessions instead of GORM's default sessions.
func (session Session) TableName() string {
	return "Sessions"
}

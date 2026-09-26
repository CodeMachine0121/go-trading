package entities

import "time"

// Session is one sign-in on one device; it exists because access tokens are stateless and cannot be revoked on their own.
type Session struct {
	ID     uint `gorm:"primaryKey"`
	UserID uint `gorm:"not null;index"`
	// ChainID is shared by every session one sign-in has been renewed into, so ending the chain is one query.
	ChainID string `gorm:"size:64;not null;index"`
	// RefreshTokenDigest is unsalted so the row can be looked up by it; see RandomRefreshTokenProxy for why that is safe.
	RefreshTokenDigest string    `gorm:"size:64;not null;uniqueIndex:idx_sessions_refresh_token_digest"`
	ExpiresAt          time.Time `gorm:"type:timestamptz;not null"`
	// RevokedAt nil means the session is still valid.
	RevokedAt *time.Time `gorm:"type:timestamptz"`
	CreatedAt time.Time  `gorm:"type:timestamptz;not null"`
	// ConnectorClientIdentifier and Audience are empty for web sign-ins.
	ConnectorClientIdentifier string `gorm:"size:64;not null;default:''"`
	Audience                  string `gorm:"size:2048;not null;default:''"`
}

func (session Session) TableName() string {
	return "Sessions"
}

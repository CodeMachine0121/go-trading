package entities

import "time"

// ConnectorAuthorizationCode stores only the code's digest; SessionChainID is empty until the code is exchanged, then names the chain torn down if the code is replayed.
type ConnectorAuthorizationCode struct {
	ID                        uint      `gorm:"primaryKey"`
	CodeDigest                string    `gorm:"size:64;not null;uniqueIndex:idx_connector_authorization_codes_code_digest"`
	UserID                    uint      `gorm:"not null;index"`
	ConnectorClientIdentifier string    `gorm:"size:64;not null;index"`
	RedirectUri               string    `gorm:"size:2048;not null"`
	CodeChallenge             string    `gorm:"size:128;not null"`
	Resource                  string    `gorm:"size:2048;not null;default:''"`
	ExpiresAt                 time.Time `gorm:"type:timestamptz;not null"`
	SessionChainID            string    `gorm:"size:64;not null;default:''"`
	CreatedAt                 time.Time `gorm:"type:timestamptz;not null"`
}

func (connectorAuthorizationCode ConnectorAuthorizationCode) TableName() string {
	return "ConnectorAuthorizationCodes"
}

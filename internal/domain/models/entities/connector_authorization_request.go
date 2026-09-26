package entities

import "time"

// ConnectorAuthorizationRequest waits for the user's decision; RedirectUri is kept exactly as the connector sent it.
type ConnectorAuthorizationRequest struct {
	ID                        uint       `gorm:"primaryKey"`
	RequestIdentifier         string     `gorm:"size:64;not null;uniqueIndex:idx_connector_authorization_requests_request_identifier"`
	ConnectorClientIdentifier string     `gorm:"size:64;not null;index"`
	RedirectUri               string     `gorm:"size:2048;not null"`
	CodeChallenge             string     `gorm:"size:128;not null"`
	State                     string     `gorm:"size:2048;not null;default:''"`
	Resource                  string     `gorm:"size:2048;not null;default:''"`
	ExpiresAt                 time.Time  `gorm:"type:timestamptz;not null"`
	DecidedAt                 *time.Time `gorm:"type:timestamptz"`
	CreatedAt                 time.Time  `gorm:"type:timestamptz;not null"`
}

func (connectorAuthorizationRequest ConnectorAuthorizationRequest) TableName() string {
	return "ConnectorAuthorizationRequests"
}

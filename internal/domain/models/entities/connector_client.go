package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ConnectorClient holds no secret: connectors run on the user's own machine and could not keep one.
type ConnectorClient struct {
	ID                    uint                            `gorm:"primaryKey"`
	ClientIdentifier      string                          `gorm:"size:64;not null;uniqueIndex:idx_connector_clients_client_identifier"`
	ClientName            string                          `gorm:"size:512;not null"`
	RedirectUris          []string                        `gorm:"serializer:json;type:jsonb;not null"`
	CreatedAt             time.Time                       `gorm:"type:timestamptz;not null"`
	AuthorizationRequests []ConnectorAuthorizationRequest `gorm:"foreignKey:ConnectorClientIdentifier;references:ClientIdentifier;constraint:OnDelete:CASCADE"`
	AuthorizationCodes    []ConnectorAuthorizationCode    `gorm:"foreignKey:ConnectorClientIdentifier;references:ClientIdentifier;constraint:OnDelete:CASCADE"`
}

func (connectorClient ConnectorClient) TableName() string {
	return "ConnectorClients"
}

func (connectorClient ConnectorClient) ToDto() dto.ConnectorClientDto {
	return dto.ConnectorClientDto{
		ClientIdentifier:         connectorClient.ClientIdentifier,
		ClientName:               connectorClient.ClientName,
		RedirectUris:             connectorClient.RedirectUris,
		GrantTypes:               []string{"authorization_code", "refresh_token"},
		ResponseTypes:            []string{"code"},
		TokenEndpointAuthMethod:  "none",
		ClientIdentifierIssuedAt: connectorClient.CreatedAt.Unix(),
	}
}

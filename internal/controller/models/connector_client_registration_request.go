package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// ConnectorClientRegistrationRequest follows RFC 7591, hence snake_case.
type ConnectorClientRegistrationRequest struct {
	RedirectUris            []string `json:"redirect_uris"`
	ClientName              string   `json:"client_name"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

func (connectorClientRegistrationRequest ConnectorClientRegistrationRequest) ToRegistrationDto() dto.ConnectorClientRegistrationDto {
	return dto.ConnectorClientRegistrationDto{
		RedirectUris:            connectorClientRegistrationRequest.RedirectUris,
		ClientName:              connectorClientRegistrationRequest.ClientName,
		GrantTypes:              connectorClientRegistrationRequest.GrantTypes,
		ResponseTypes:           connectorClientRegistrationRequest.ResponseTypes,
		TokenEndpointAuthMethod: connectorClientRegistrationRequest.TokenEndpointAuthMethod,
	}
}

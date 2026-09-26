package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// ConnectorTokenRequest is the form body of the OAuth token endpoint; which fields matter depends on the grant type.
type ConnectorTokenRequest struct {
	GrantType        string `form:"grant_type"`
	Code             string `form:"code"`
	RedirectUri      string `form:"redirect_uri"`
	ClientIdentifier string `form:"client_id"`
	CodeVerifier     string `form:"code_verifier"`
	RefreshToken     string `form:"refresh_token"`
}

func (connectorTokenRequest ConnectorTokenRequest) ToCodeExchangeDto() dto.ConnectorAuthorizationCodeExchangeDto {
	return dto.ConnectorAuthorizationCodeExchangeDto{
		Code:             connectorTokenRequest.Code,
		RedirectUri:      connectorTokenRequest.RedirectUri,
		ClientIdentifier: connectorTokenRequest.ClientIdentifier,
		CodeVerifier:     connectorTokenRequest.CodeVerifier,
	}
}

func (connectorTokenRequest ConnectorTokenRequest) ToRenewalDto() dto.SessionRenewalDto {
	return dto.SessionRenewalDto{
		RefreshToken:              connectorTokenRequest.RefreshToken,
		ConnectorClientIdentifier: connectorTokenRequest.ClientIdentifier,
	}
}

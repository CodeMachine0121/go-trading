package dto

// ConnectorClientDto is the RFC 7591 registration response, hence snake_case.
type ConnectorClientDto struct {
	ClientIdentifier         string   `json:"client_id"`
	ClientName               string   `json:"client_name"`
	RedirectUris             []string `json:"redirect_uris"`
	GrantTypes               []string `json:"grant_types"`
	ResponseTypes            []string `json:"response_types"`
	TokenEndpointAuthMethod  string   `json:"token_endpoint_auth_method"`
	ClientIdentifierIssuedAt int64    `json:"client_id_issued_at"`
}

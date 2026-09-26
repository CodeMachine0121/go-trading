package dto

// ConnectorClientRegistrationDto keeps the optional lists nil when the connector left them out.
type ConnectorClientRegistrationDto struct {
	RedirectUris            []string
	ClientName              string
	GrantTypes              []string
	ResponseTypes           []string
	TokenEndpointAuthMethod string
}

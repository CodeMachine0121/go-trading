package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// ConnectorAuthorizationPolicyVo takes public addresses from configuration because the proxies in front rewrite the scheme.
type ConnectorAuthorizationPolicyVo struct {
	PublicBaseUrl   string
	FrontendBaseUrl string
	RequestLifetime time.Duration
	CodeLifetime    time.Duration
}

func (connectorAuthorizationPolicyVo ConnectorAuthorizationPolicyVo) ToServerMetadataDto() dto.ConnectorAuthorizationServerMetadataDto {
	base := connectorAuthorizationPolicyVo.PublicBaseUrl

	return dto.ConnectorAuthorizationServerMetadataDto{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/oauth/authorize",
		TokenEndpoint:                     base + "/oauth/token",
		RegistrationEndpoint:              base + "/oauth/register",
		IntrospectionEndpoint:             base + "/oauth/introspection",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
	}
}

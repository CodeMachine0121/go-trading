package domains

import (
	"net/url"
	"regexp"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

const (
	connectorCodeChallengeMethod    = "S256"
	connectorEchoedValueLengthLimit = 2048
)

// connectorCodeChallengeShape is an unpadded base64url SHA-256 digest, the only challenge S256 can produce (RFC 7636 §4.2).
var connectorCodeChallengeShape = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// ConnectorAuthorizationStartDomain is built only after the redirect address is trusted, so every refusal here may be sent back to it.
type ConnectorAuthorizationStartDomain struct {
	startDto    dto.ConnectorAuthorizationStartDto
	redirectUri ConnectorRedirectUriDomain
}

func NewConnectorAuthorizationStartDomain(
	startDto dto.ConnectorAuthorizationStartDto, redirectUri ConnectorRedirectUriDomain,
) ConnectorAuthorizationStartDomain {
	return ConnectorAuthorizationStartDomain{startDto: startDto, redirectUri: redirectUri}
}

// RefusalRedirect sends the browser back to the connector when the request is incomplete; scope is deliberately not looked at.
func (connectorAuthorizationStartDomain ConnectorAuthorizationStartDomain) RefusalRedirect() (
	dto.ConnectorAuthorizationRedirectDto, bool,
) {
	startDto := connectorAuthorizationStartDomain.startDto
	description := ""
	switch {
	case startDto.ResponseType != connectorCodeResponseType:
		description = "只支援授權碼（response_type=code）"
	case startDto.CodeChallenge == "":
		description = "缺少挑戰（code_challenge）"
	case startDto.CodeChallengeMethod != connectorCodeChallengeMethod:
		description = "挑戰方式只支援 S256"
	case !connectorCodeChallengeShape.MatchString(startDto.CodeChallenge):
		description = "挑戰（code_challenge）必須是 43 字元的 base64url"
	case len(startDto.State) > connectorEchoedValueLengthLimit ||
		len(startDto.Resource) > connectorEchoedValueLengthLimit:
		description = "請求內容過長"
	default:
		return dto.ConnectorAuthorizationRedirectDto{}, false
	}

	parameters := url.Values{
		"error":             {"invalid_request"},
		"error_description": {description},
	}
	if startDto.State != "" {
		parameters.Set("state", startDto.State)
	}

	return dto.ConnectorAuthorizationRedirectDto{
		RedirectTo: connectorAuthorizationStartDomain.redirectUri.WithParameters(parameters),
	}, true
}

func (connectorAuthorizationStartDomain ConnectorAuthorizationStartDomain) ToEntity(
	requestIdentifier string, now time.Time, lifetime time.Duration,
) entities.ConnectorAuthorizationRequest {
	startDto := connectorAuthorizationStartDomain.startDto

	return entities.ConnectorAuthorizationRequest{
		RequestIdentifier:         requestIdentifier,
		ConnectorClientIdentifier: startDto.ClientIdentifier,
		RedirectUri:               startDto.RedirectUri,
		CodeChallenge:             startDto.CodeChallenge,
		State:                     startDto.State,
		Resource:                  startDto.Resource,
		ExpiresAt:                 now.Add(lifetime).UTC(),
		CreatedAt:                 now.UTC(),
	}
}

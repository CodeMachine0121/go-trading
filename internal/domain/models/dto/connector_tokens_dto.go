package dto

// ConnectorTokensDto is the OAuth token response, hence snake_case.
type ConnectorTokensDto struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
}

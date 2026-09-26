package dto

type ConnectorAuthorizationCodeExchangeDto struct {
	Code             string
	RedirectUri      string
	ClientIdentifier string
	CodeVerifier     string
}

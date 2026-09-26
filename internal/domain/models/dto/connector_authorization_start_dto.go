package dto

type ConnectorAuthorizationStartDto struct {
	ResponseType        string
	ClientIdentifier    string
	RedirectUri         string
	CodeChallenge       string
	CodeChallengeMethod string
	State               string
	Resource            string
}

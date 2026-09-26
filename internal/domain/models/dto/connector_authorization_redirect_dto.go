package dto

// ConnectorAuthorizationRedirectDto is where the browser goes next.
type ConnectorAuthorizationRedirectDto struct {
	RedirectTo string `json:"redirectTo"`
}

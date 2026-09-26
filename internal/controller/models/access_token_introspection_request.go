package models

// AccessTokenIntrospectionRequest is the RFC 7662 form body.
type AccessTokenIntrospectionRequest struct {
	Token string `form:"token"`
}

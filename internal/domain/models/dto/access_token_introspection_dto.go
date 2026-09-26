package dto

// AccessTokenIntrospectionDto follows RFC 7662; an inactive answer carries nothing but active=false.
type AccessTokenIntrospectionDto struct {
	Active    bool    `json:"active"`
	Subject   *string `json:"sub,omitempty"`
	Audience  *string `json:"aud,omitempty"`
	ExpiresAt *int64  `json:"exp,omitempty"`
}

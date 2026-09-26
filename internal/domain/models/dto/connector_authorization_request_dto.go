package dto

import "time"

type ConnectorAuthorizationRequestDto struct {
	ClientName string    `json:"clientName"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

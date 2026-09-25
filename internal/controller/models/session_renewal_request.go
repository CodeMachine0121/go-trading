package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// SessionRenewalRequest serves both renewing and ending a session, since both take the same renewal proof.
type SessionRenewalRequest struct {
	RefreshToken string `json:"refreshToken"`
}

func (sessionRenewalRequest SessionRenewalRequest) ToRenewalDto() dto.SessionRenewalDto {
	return dto.SessionRenewalDto{RefreshToken: sessionRenewalRequest.RefreshToken}
}

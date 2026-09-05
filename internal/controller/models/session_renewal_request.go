package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// SessionRenewalRequest is the body a caller sends to renew a session or to end one.
//
// One shape serves both, which is unusual here — the other request types are
// deliberately separate even when their fields match. These two are not two rules
// wearing the same shape: they are literally the same input, a renewal proof, and
// the only thing that differs is which of the two things is done with it.
type SessionRenewalRequest struct {
	RefreshToken string `json:"refreshToken"`
}

// ToRenewalDto turns the request into the shape the domain accepts. The proof is
// handed on untouched: what counts as an unusable one is the domain's rule.
func (sessionRenewalRequest SessionRenewalRequest) ToRenewalDto() dto.SessionRenewalDto {
	return dto.SessionRenewalDto{RefreshToken: sessionRenewalRequest.RefreshToken}
}

package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// SignInRequest is the body a caller sends to sign in.
//
// It is a separate type from UserRegistrationRequest despite holding the same two
// fields, for the same reason the two DTOs are separate: they are judged by
// different rules, and one shape shared between them is how the rules end up shared
// too.
type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ToSignInDto turns the request into the shape the domain accepts.
func (signInRequest SignInRequest) ToSignInDto() dto.SignInDto {
	return dto.SignInDto{
		Email:    signInRequest.Email,
		Password: signInRequest.Password,
	}
}

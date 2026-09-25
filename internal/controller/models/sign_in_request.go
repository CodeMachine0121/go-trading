package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// SignInRequest is kept separate from UserRegistrationRequest despite identical fields because they are judged by different rules.
type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (signInRequest SignInRequest) ToSignInDto() dto.SignInDto {
	return dto.SignInDto{
		Email:    signInRequest.Email,
		Password: signInRequest.Password,
	}
}

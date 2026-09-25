package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// UserRegistrationRequest marks no field binding-required so the domain, not the binder, words every refusal.
type UserRegistrationRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ToRegistrationDto passes both fields on untouched; trimming and password rules belong to the domain.
func (userRegistrationRequest UserRegistrationRequest) ToRegistrationDto() dto.UserRegistrationDto {
	return dto.UserRegistrationDto{
		Email:    userRegistrationRequest.Email,
		Password: userRegistrationRequest.Password,
	}
}

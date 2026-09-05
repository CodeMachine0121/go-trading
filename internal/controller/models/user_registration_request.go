package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// UserRegistrationRequest is the body a caller sends to create a user.
//
// Neither field is marked as required for the binder to enforce. What counts as an
// acceptable address and an acceptable password is a list of rules the domain owns,
// and a binder that refuses an empty one first would answer in its own words —
// English, structural, and about a JSON field rather than about an account.
type UserRegistrationRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ToRegistrationDto turns the request into the shape the domain accepts. Both halves
// are handed on untouched: trimming the address and judging the password are rules,
// and rules do not live in this layer.
func (userRegistrationRequest UserRegistrationRequest) ToRegistrationDto() dto.UserRegistrationDto {
	return dto.UserRegistrationDto{
		Email:    userRegistrationRequest.Email,
		Password: userRegistrationRequest.Password,
	}
}

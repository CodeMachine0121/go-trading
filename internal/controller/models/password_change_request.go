package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// PasswordChangeRequest carries no user ID (the account comes from the token), and neither field is binding-required so the domain can word each refusal.
type PasswordChangeRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (passwordChangeRequest PasswordChangeRequest) ToPasswordChangeDto() dto.PasswordChangeDto {
	return dto.PasswordChangeDto{
		CurrentPassword: passwordChangeRequest.CurrentPassword,
		NewPassword:     passwordChangeRequest.NewPassword,
	}
}

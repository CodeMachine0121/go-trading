package models

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// PasswordChangeRequest is the body of a request to replace a password.
//
// It carries the two passwords and nothing else. In particular it carries no user
// identifier: which account this changes is decided by the proof of identity on the
// request, and a field here would be a field somebody could fill in with somebody
// else's.
//
// Neither field is marked required. A blank current password is a current password
// that will not match, and a blank new one breaks a rule the domain already words
// better than a binding error does — and both refusals belong to whichever box the
// person has to go back and fix.
type PasswordChangeRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ToPasswordChangeDto is this request in the shape the domain accepts.
func (passwordChangeRequest PasswordChangeRequest) ToPasswordChangeDto() dto.PasswordChangeDto {
	return dto.PasswordChangeDto{
		CurrentPassword: passwordChangeRequest.CurrentPassword,
		NewPassword:     passwordChangeRequest.NewPassword,
	}
}

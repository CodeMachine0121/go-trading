package dto

// UserDto deliberately has no password or password hash field.
type UserDto struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
	// IsEnabled is returned on every answer so a pending user can see when they have been let in.
	IsEnabled bool `json:"isEnabled"`
	// ActivationInstruction is a pointer so it disappears once the user is enabled.
	ActivationInstruction *AccountActivationInstructionDto `json:"activationInstruction,omitempty"`
}

package dto

// PasswordChangeDto has no user field because the user always comes from the request's credentials.
type PasswordChangeDto struct {
	CurrentPassword string
	NewPassword     string
}

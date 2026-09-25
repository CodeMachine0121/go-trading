package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// PasswordChangeDomain reuses PasswordDomain's rules and adds only that the new password must differ from the current one.
type PasswordChangeDomain struct {
	currentPassword string
	newPassword     PasswordDomain
}

// NewPasswordChangeDomain judges the new password before the pair so the refusal is the one that must be fixed regardless; the current password is not judged, since it may predate current rules.
func NewPasswordChangeDomain(passwordChangeDto dto.PasswordChangeDto) (PasswordChangeDomain, error) {
	newPassword, passwordError := NewPasswordDomain(passwordChangeDto.NewPassword)
	if passwordError != nil {
		return PasswordChangeDomain{}, passwordError
	}

	// Compared exactly as typed, since blanks and capitals are part of a password.
	if passwordChangeDto.CurrentPassword == passwordChangeDto.NewPassword {
		return PasswordChangeDomain{}, fmt.Errorf(
			"%w: 新密碼不得與目前的密碼相同", ErrUserValidation)
	}

	return PasswordChangeDomain{
		currentPassword: passwordChangeDto.CurrentPassword,
		newPassword:     newPassword,
	}, nil
}

// CurrentPassword is to be checked against the stored proof and kept nowhere.
func (passwordChangeDomain PasswordChangeDomain) CurrentPassword() string {
	return passwordChangeDomain.currentPassword
}

// NewPassword is to be hashed into the replacement proof and kept nowhere.
func (passwordChangeDomain PasswordChangeDomain) NewPassword() string {
	return passwordChangeDomain.newPassword.Value()
}

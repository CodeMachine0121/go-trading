package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// PasswordChangeDomain holds one attempt to replace a password and guarantees every
// rule about the replacement passed. An instance only exists when it did.
//
// The rules about what a password may be are not restated here — PasswordDomain
// already owns them, and owning them twice is how the two lists start disagreeing
// the day one of them is edited. What this model adds is the one rule that only
// exists when there is an old password to compare against: the new one must not be
// the old one.
//
// It knows nothing about proofs, storage, or sessions. It sees two strings and says
// whether they are an acceptable pair.
type PasswordChangeDomain struct {
	currentPassword string
	newPassword     PasswordDomain
}

// NewPasswordChangeDomain judges the new password first and the pair second.
//
// The order is the same idea as UserRegistrationDomain's: when both are wrong, the
// refusal that comes back should be the one the person has to fix regardless. A new
// password that is too short is too short whether or not it also happens to equal
// the old one, so saying "they are the same" first would have somebody pick a
// different password only to be told it was never long enough.
//
// The current password is not judged at all. It is not being set — it is being
// checked against what is stored, and only the store can do that. Judging it here
// would refuse somebody whose real password predates a rule that has since changed.
func NewPasswordChangeDomain(passwordChangeDto dto.PasswordChangeDto) (PasswordChangeDomain, error) {
	newPassword, passwordError := NewPasswordDomain(passwordChangeDto.NewPassword)
	if passwordError != nil {
		return PasswordChangeDomain{}, passwordError
	}

	// Compared exactly as typed, with nothing trimmed or lowered. A password's
	// blanks and capitals are part of it, so "correct horse" and "Correct horse"
	// really are two different passwords and changing between them really is a
	// change.
	if passwordChangeDto.CurrentPassword == passwordChangeDto.NewPassword {
		return PasswordChangeDomain{}, fmt.Errorf(
			"%w: 新密碼不得與目前的密碼相同", ErrUserValidation)
	}

	return PasswordChangeDomain{
		currentPassword: passwordChangeDto.CurrentPassword,
		newPassword:     newPassword,
	}, nil
}

// CurrentPassword is the password to be checked against what is stored. It is read
// once and kept nowhere.
func (passwordChangeDomain PasswordChangeDomain) CurrentPassword() string {
	return passwordChangeDomain.currentPassword
}

// NewPassword is the password to be turned into the proof that replaces the stored
// one. It is read once and kept nowhere.
func (passwordChangeDomain PasswordChangeDomain) NewPassword() string {
	return passwordChangeDomain.newPassword.Value()
}

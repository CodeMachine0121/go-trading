package domains

import (
	"fmt"
	"unicode/utf8"
)

// passwordMinimumLength is how many characters a password must have. It is counted
// in characters rather than bytes because that is what a person typing one counts.
const passwordMinimumLength = 8

// passwordMaximumByteLength is how many bytes a password may take up, which is a
// different question from how many characters it has: eight Chinese characters are
// twenty-four bytes.
//
// The limit is not this system's preference. Every password hashing scheme has some
// ceiling, and the one in use stops reading at seventy-two bytes — so a longer
// password is not a longer password, it is the first seventy-two bytes of one. The
// number lives here, in the rules, rather than inside the hashing implementation,
// because whoever types a seventy-three byte password has to be told so, and only
// the rules get to say things to people.
const passwordMaximumByteLength = 72

// PasswordDomain holds one password and guarantees it is long enough to be worth
// having and short enough to be used whole. An instance only exists when both rules
// passed.
//
// It does not trim, lower, or otherwise touch what it was given. Every other piece
// of text in this system gets tidied up on the way in; a password must not, because
// the blanks and the capitals are part of it. Tidying it would mean a password that
// works today stops working the day the tidying changes.
type PasswordDomain struct {
	value string
}

// NewPasswordDomain judges a password against both of its limits.
//
// Being too long is a refusal rather than a trim, and that is the important half.
// Trimming it would let somebody set a long password, be told it worked, and go away
// believing they have a long password — while what actually guards their account is
// the front of it. They would never find out.
func NewPasswordDomain(password string) (PasswordDomain, error) {
	if password == "" {
		return PasswordDomain{}, fmt.Errorf("%w: 必須給一組密碼", ErrUserValidation)
	}

	if utf8.RuneCountInString(password) < passwordMinimumLength {
		return PasswordDomain{}, fmt.Errorf(
			"%w: 密碼至少要 %d 個字元", ErrUserValidation, passwordMinimumLength)
	}

	if len(password) > passwordMaximumByteLength {
		return PasswordDomain{}, fmt.Errorf(
			"%w: 密碼長度上限為 %d 個位元組（中文字一個算三個）",
			ErrUserValidation, passwordMaximumByteLength)
	}

	return PasswordDomain{value: password}, nil
}

// Value is the password exactly as it was typed. It is read once, to be turned into
// a proof, and is not stored anywhere afterwards.
func (passwordDomain PasswordDomain) Value() string {
	return passwordDomain.value
}

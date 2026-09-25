package domains

import (
	"fmt"
	"unicode/utf8"
)

// passwordMinimumLength counts characters, not bytes.
const passwordMinimumLength = 8

// passwordMaximumByteLength is the hashing scheme's 72-byte limit, enforced here so users are told rather than silently truncated.
const passwordMaximumByteLength = 72

// PasswordDomain never trims or lowercases, because blanks and capitals are part of a password.
type PasswordDomain struct {
	value string
}

// NewPasswordDomain refuses an over-long password rather than trimming it, so nobody believes a longer password guards their account than actually does.
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

// Value is exactly as typed and is not stored after being hashed.
func (passwordDomain PasswordDomain) Value() string {
	return passwordDomain.value
}

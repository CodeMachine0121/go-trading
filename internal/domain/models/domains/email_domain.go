package domains

import (
	"fmt"
	"net/mail"
	"strings"
)

// emailMaxLength is the longest deliverable email address.
const emailMaxLength = 320

// EmailDomain is the single place an address is normalised, so registration and sign-in can never disagree on spelling.
// The whole address, local part included, is lowercased by convention even though the standard allows case-sensitive local parts.
type EmailDomain struct {
	value string
}

// NewEmailDomain trims and lowercases before judging, so an all-blank address is reported as missing rather than malformed.
func NewEmailDomain(email string) (EmailDomain, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return EmailDomain{}, fmt.Errorf("%w: 必須給一個電子郵件", ErrUserValidation)
	}

	if strings.ContainsRune(normalizedEmail, nulCharacter) {
		return EmailDomain{}, fmt.Errorf(
			"%w: 電子郵件不得包含空字元（NUL）", ErrUserValidation)
	}

	if len(normalizedEmail) > emailMaxLength {
		return EmailDomain{}, fmt.Errorf(
			"%w: 電子郵件長度上限為 %d 個位元組", ErrUserValidation, emailMaxLength)
	}

	// Use the standard parser, but require the parsed address to equal the input so display-name forms like `Name <a@b>` are refused.
	parsedAddress, parseError := mail.ParseAddress(normalizedEmail)
	if parseError != nil || parsedAddress.Address != normalizedEmail {
		return EmailDomain{}, fmt.Errorf(
			"%w: 「%s」不是一個電子郵件的格式", ErrUserValidation, normalizedEmail)
	}

	return EmailDomain{value: normalizedEmail}, nil
}

// Value is trimmed and lowercased.
func (emailDomain EmailDomain) Value() string {
	return emailDomain.value
}

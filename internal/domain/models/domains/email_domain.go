package domains

import (
	"fmt"
	"net/mail"
	"strings"
)

// emailMaxLength is how long an email address may be. It is the longest an address
// can be and still be deliverable, so it is a fact about email rather than a choice
// this system made — but it has to be written down somewhere, because a column has
// to be some width and an unbounded one is a way to fill a disk.
const emailMaxLength = 320

// EmailDomain holds one email address and guarantees it is one. An instance only
// exists when every rule passed, so there is no half-valid address further in.
//
// It is the single place the address is normalised, and that is the whole reason it
// exists as a model rather than a check inside each caller. Registering and signing
// in both go through it, so it is not possible for an account to be created under
// one spelling and then be unreachable under another — which is exactly what two
// copies of "trim it and lower it" grow into the first time one of them is edited.
//
// Lowering the whole address, local part included, is a practical convention rather
// than what the standard permits: the standard lets a mail server treat Bob@ and
// bob@ as two people, and no mail server anybody uses does. Treating them as two
// people here would mean a person who capitalises their own name on Tuesday cannot
// sign in on Tuesday.
type EmailDomain struct {
	value string
}

// NewEmailDomain reads an address, dropping the blanks around it and lowering it
// before judging it. The order matters: an address made only of blanks has to be
// refused as "you did not give one" rather than as "that is not an address", because
// those two send whoever reads them looking in different places.
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

	// The parser is the standard's, not one written here: an address is a small
	// grammar with a long tail of legal shapes, and a pattern that looks right is
	// how a system ends up refusing somebody's real address forever.
	//
	// What it accepts is wider than what an account may be, though. It reads a whole
	// address line, so `James Hsueh <james@example.com>` parses happily — and an
	// account is the address itself, not a line out of an address book. Insisting
	// that the address it found back is the whole of what arrived is what narrows
	// the standard's answer to this system's question.
	parsedAddress, parseError := mail.ParseAddress(normalizedEmail)
	if parseError != nil || parsedAddress.Address != normalizedEmail {
		return EmailDomain{}, fmt.Errorf(
			"%w: 「%s」不是一個電子郵件的格式", ErrUserValidation, normalizedEmail)
	}

	return EmailDomain{value: normalizedEmail}, nil
}

// Value is the address as it is stored and compared: trimmed and lowered.
func (emailDomain EmailDomain) Value() string {
	return emailDomain.value
}

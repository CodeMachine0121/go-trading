package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// User is one person this system recognises: an identity, the email address that
// doubles as the account, and a proof derived from the password. It is a plain data
// model — fields, persistence mapping and shape conversion only, no business rules.
//
// The password itself is deliberately absent. It exists for the length of one
// request and is never written anywhere, so there is no field here that could leak
// it, no query that could return it, and nothing for a stolen dump to give away.
//
// The email carries a unique index because it is what a person signs in as, and the
// index — not a read-then-write check — is what actually makes it unique: two
// registrations arriving at once both pass a check, and only one passes an index.
type User struct {
	ID uint `gorm:"primaryKey"`
	// Email is always stored already trimmed and lowered (see EmailDomain), so the
	// index is over the one spelling the system ever compares.
	Email string `gorm:"size:320;not null;uniqueIndex:idx_users_email"`
	// PasswordProof is what is kept instead of the password: derived from it,
	// salted, and not reversible. Two people who happen to choose the same password
	// leave two different proofs behind.
	PasswordProof string    `gorm:"size:255;not null"`
	CreatedAt     time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt     time.Time `gorm:"type:timestamptz;not null"`
}

// TableName pins the table to Users instead of GORM's default users.
func (user User) TableName() string {
	return "Users"
}

// ToDto converts this record into the shape the domain hands outwards.
//
// There is no line here dropping the password proof, because UserDto has nowhere to
// put one. "The answer never carries the proof" is therefore something the types
// make impossible rather than something a reviewer has to keep checking.
func (user User) ToDto() dto.UserDto {
	return dto.UserDto{
		ID:    user.ID,
		Email: user.Email,
	}
}

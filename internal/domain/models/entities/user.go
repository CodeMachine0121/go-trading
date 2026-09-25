package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// User is one account; the password itself is never stored, only a salted irreversible proof.
type User struct {
	ID uint `gorm:"primaryKey"`
	// Email is stored trimmed and lowercased (see EmailDomain), so the unique index covers the one spelling ever compared.
	Email         string `gorm:"size:320;not null;uniqueIndex:idx_users_email"`
	PasswordProof string `gorm:"size:255;not null"`
	// IsEnabled defaults to false and nothing in the system writes it, so users cannot let themselves in; activation happens outside.
	IsEnabled bool `gorm:"not null;default:false"`
	// FailedSignInCount counts consecutive wrong passwords for lockout.
	FailedSignInCount int `gorm:"not null;default:0"`
	// LockedUntil is nil when never locked, distinct from a lock that has already passed.
	LockedUntil *time.Time `gorm:"type:timestamptz"`
	CreatedAt   time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt   time.Time  `gorm:"type:timestamptz;not null"`
	// Sessions cascade so a deleted user has no sessions.
	Sessions []Session `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	// TelegramDelivery cascades so a bot key never outlives its owner.
	TelegramDelivery *TelegramDelivery `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

func (user User) TableName() string {
	return "Users"
}

// ToDto cannot leak the password proof because UserDto has no field for it; the activation instruction is added by AccountActivationDomain.
func (user User) ToDto() dto.UserDto {
	return dto.UserDto{
		ID:        user.ID,
		Email:     user.Email,
		IsEnabled: user.IsEnabled,
	}
}

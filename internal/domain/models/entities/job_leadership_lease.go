package entities

import "time"

// JobLeadershipLease is the one row that says which replica is on duty for work the whole system needs only once.
type JobLeadershipLease struct {
	// Name keys the lease so a second, independent duty could take its own row later.
	Name       string    `gorm:"primaryKey;size:64"`
	HolderName string    `gorm:"size:255;not null"`
	ExpiresAt  time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt  time.Time `gorm:"type:timestamptz;not null"`
}

func (jobLeadershipLease JobLeadershipLease) TableName() string {
	return "JobLeadershipLeases"
}

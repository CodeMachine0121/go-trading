package entities

import "time"

// AppliedDataRetirement is the record that one body of stored data has already been
// retired. It is a plain data model: fields and persistence mapping only.
//
// The name is the key because the name is the identity. A retirement is a thing that
// happened once, and what makes two runs the same run is that they are retiring the
// same data — not when they ran or in what order.
type AppliedDataRetirement struct {
	Name      string    `gorm:"primaryKey;size:128"`
	AppliedAt time.Time `gorm:"type:timestamptz;not null"`
}

// TableName pins the table to AppliedDataRetirements, matching how every other table
// in this schema is spelled.
func (appliedDataRetirement AppliedDataRetirement) TableName() string {
	return "AppliedDataRetirements"
}

package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// SymbolScreeningRun is the queued work the duty replica claims; the snapshot keeps it independent of later profile or script edits.
type SymbolScreeningRun struct {
	ID          uint                           `gorm:"primaryKey;index:idx_symbol_screening_runs_owner_id_id,priority:2,sort:desc;index:idx_symbol_screening_runs_status_id,priority:2"`
	OwnerID     uint                           `gorm:"not null;index:idx_symbol_screening_runs_owner_id_id,priority:1"`
	ProfileID   *uint                          `gorm:"index"`
	ProfileName string                         `gorm:"size:128;not null"`
	Snapshot    dto.SymbolScreeningSnapshotDto `gorm:"serializer:json;type:jsonb;not null"`
	Status      string                         `gorm:"size:16;not null;index:idx_symbol_screening_runs_status_id,priority:1"`
	// ProcessedCandidateCount counts candidates that reached a verdict, so progress never runs backwards.
	ProcessedCandidateCount int    `gorm:"not null;default:0"`
	CandidateCount          int    `gorm:"not null"`
	ReplicaName             string `gorm:"size:255;not null;default:''"`
	FailureReason           string `gorm:"type:text;not null;default:''"`
	// ContributionFailureReason says why the baseline could not be compared, without failing the run.
	ContributionFailureReason string     `gorm:"type:text;not null;default:''"`
	SubmittedAt               time.Time  `gorm:"type:timestamptz;not null"`
	StartedAt                 *time.Time `gorm:"type:timestamptz"`
	FinishedAt                *time.Time `gorm:"type:timestamptz;index"`

	Owner      User                       `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	Profile    *SymbolScreeningProfile    `gorm:"foreignKey:ProfileID;constraint:OnDelete:SET NULL"`
	Candidates []SymbolScreeningCandidate `gorm:"foreignKey:RunID;constraint:OnDelete:CASCADE"`
}

func (symbolScreeningRun SymbolScreeningRun) TableName() string {
	return "SymbolScreeningRuns"
}

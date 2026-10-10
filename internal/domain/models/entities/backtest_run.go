package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// BacktestRun is written when a replay request is accepted, so the row itself is the work that replicas claim, callers poll and a restart sweeps up.
type BacktestRun struct {
	ID        uint      `gorm:"primaryKey"`
	OwnerID   uint      `gorm:"not null;index"`
	Kind      string    `gorm:"size:32;not null"`
	Symbol    string    `gorm:"size:64;not null"`
	Interval  string    `gorm:"size:8;not null"`
	StartTime time.Time `gorm:"type:timestamptz;not null"`
	EndTime   time.Time `gorm:"type:timestamptz;not null"`
	// Leverage and TradingMode are empty for spot replays.
	Leverage    decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TradingMode string              `gorm:"size:16;not null;default:''"`
	// StrategyScriptID and TradingStrategyID let a page list the runs of one script or one strategy.
	StrategyScriptID  *uint  `gorm:"index"`
	TradingStrategyID *uint  `gorm:"index"`
	SubjectName       string `gorm:"type:text;not null;default:''"`
	// Request is the resolved request, scripts included, so a later edit of the script cannot change what this run replays.
	Request dto.BacktestRunRequestDto `gorm:"serializer:json;type:jsonb;not null"`
	Status  string                    `gorm:"size:16;not null;index"`
	// CompletedBarCount over TotalBarCount is the progress; both stay zero while queued.
	CompletedBarCount int `gorm:"not null;default:0"`
	TotalBarCount     int `gorm:"not null;default:0"`
	// ReplicaName is the replica running it, so only that replica vanishing marks it interrupted.
	ReplicaName   string `gorm:"size:255;not null;default:''"`
	FailureReason string `gorm:"type:text;not null;default:''"`
	// The headline figures are nil until the run succeeds, so a list never reads the report.
	TotalReturnRate *float64 `gorm:"type:double precision"`
	MaximumDrawdown *float64 `gorm:"type:double precision"`
	TradeCount      *int
	SubmittedAt     time.Time  `gorm:"type:timestamptz;not null"`
	StartedAt       *time.Time `gorm:"type:timestamptz"`
	FinishedAt      *time.Time `gorm:"type:timestamptz;index"`

	Owner  User               `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
	Result *BacktestRunResult `gorm:"foreignKey:BacktestRunID;constraint:OnDelete:CASCADE"`
}

func (backtestRun BacktestRun) TableName() string {
	return "BacktestRuns"
}

package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type StrategyBotRunRecordWriteDto struct {
	StrategyBotID uint
	RanAt         time.Time
	// Result is buy, sell, hold or conflict.
	Result string
	// PositionPlan is stored as absent rather than zeroes when HasPositionPlan is false.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
	// ReferencePrice is set only for contract rounds that offered a journal link.
	ReferencePrice        decimal.NullDecimal
	JournalLinkIdentifier string
}

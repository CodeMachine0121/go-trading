package dto

import "time"

type StrategyBotRunRecordWriteDto struct {
	StrategyBotID uint
	RanAt         time.Time
	// Result is buy, sell, hold or conflict.
	Result string
	// PositionPlan is stored as absent rather than zeroes when HasPositionPlan is false.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
}

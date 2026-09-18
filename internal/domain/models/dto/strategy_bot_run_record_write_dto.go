package dto

import "time"

// StrategyBotRunRecordWriteDto is one finished round as it arrives to be remembered.
//
// It is a shape rather than a run of arguments because three of its fields only mean
// anything together: what was suggested, and whether anything was. Passed one by one
// they could be handed over in a combination nothing would refuse — a stop price
// beside a round that suggested nothing — and the history would then say something
// that never happened.
type StrategyBotRunRecordWriteDto struct {
	StrategyBotID uint
	RanAt         time.Time
	// Result is buy, sell, hold or conflict.
	Result string
	// PositionPlan is what this round suggested putting down, and HasPositionPlan
	// whether it suggested anything at all. Most rounds suggest nothing, and that is
	// remembered as nothing rather than as zeroes.
	PositionPlan    PositionPlanDto
	HasPositionPlan bool
}

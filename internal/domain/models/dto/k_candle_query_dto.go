package dto

import "time"

type KCandleQueryDto struct {
	Symbol    string
	StartTime time.Time
	EndTime   time.Time
}

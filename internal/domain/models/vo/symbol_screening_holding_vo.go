package vo

import "time"

// SymbolScreeningHoldingVo is one stretch a position was held during the screened period.
type SymbolScreeningHoldingVo struct {
	StartTime time.Time
	EndTime   time.Time
}

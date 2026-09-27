package vo

import "time"

// TradeListFilterVo leaves Status, Symbol and Market blank and OpenedSince nil to mean no narrowing.
type TradeListFilterVo struct {
	Status      string
	Symbol      string
	Market      string
	OpenedSince *time.Time
	Limit       int
}

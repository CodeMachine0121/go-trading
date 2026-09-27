package vo

import "time"

// ContractTradeListFilterVo leaves Status and Symbol blank and OpenedSince nil to mean no narrowing.
type ContractTradeListFilterVo struct {
	Status      string
	Symbol      string
	OpenedSince *time.Time
	Limit       int
}

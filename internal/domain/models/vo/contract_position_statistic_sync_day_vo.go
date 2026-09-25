package vo

import "time"

// ContractPositionStatisticSyncDayVo is one UTC calendar day (how the archive files them) with its first and last statistic times.
type ContractPositionStatisticSyncDayVo struct {
	Day                time.Time
	FirstStatisticTime time.Time
	LastStatisticTime  time.Time
}

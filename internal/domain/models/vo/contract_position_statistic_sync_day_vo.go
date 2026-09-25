package vo

import "time"

// ContractPositionStatisticSyncDayVo is one day of position statistics a contract
// history sync walks: a calendar day in UTC, which is how the venue's archive files
// them, and the first and last statistic time that day can hold.
type ContractPositionStatisticSyncDayVo struct {
	Day                time.Time
	FirstStatisticTime time.Time
	LastStatisticTime  time.Time
}

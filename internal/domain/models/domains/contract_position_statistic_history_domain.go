package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// contractPositionStatisticDay is how long one day of position statistics is, and so
// how long one file of the venue's archive covers.
const contractPositionStatisticDay = 24 * time.Hour

// ContractPositionStatisticHistoryDomain is the stretch of position statistics one
// contract history sync walks, cut into the days it walks it in.
//
// **A day is a calendar day in UTC**, because that is how the venue's archive files
// them: one file, one day, and asking about part of one is not a question it
// answers. The stretch runs from the day the lookback reaches back into, through
// today — the same lookback the candles are synced over, so the two histories cover
// the same stretch.
//
// Today, and usually yesterday, have no file yet. They are still walked: finding that
// out costs one question, and deciding in advance which days the venue has published
// would be guessing at its schedule.
type ContractPositionStatisticHistoryDomain struct {
	days []vo.ContractPositionStatisticSyncDayVo
}

// NewContractPositionStatisticHistoryDomain settles the days against one reading of
// the clock.
func NewContractPositionStatisticHistoryDomain(
	currentTime time.Time, lookback time.Duration,
) ContractPositionStatisticHistoryDomain {
	firstDay := currentTime.UTC().Add(-lookback).Truncate(contractPositionStatisticDay)
	today := currentTime.UTC().Truncate(contractPositionStatisticDay)

	days := make([]vo.ContractPositionStatisticSyncDayVo, 0)
	for day := firstDay; !day.After(today); day = day.Add(contractPositionStatisticDay) {
		days = append(days, vo.ContractPositionStatisticSyncDayVo{
			Day:                day,
			FirstStatisticTime: day,
			LastStatisticTime:  day.Add(contractPositionStatisticDay - ContractPositionStatisticInterval),
		})
	}

	return ContractPositionStatisticHistoryDomain{days: days}
}

// Days are the days to walk, oldest first.
func (historyDomain ContractPositionStatisticHistoryDomain) Days() []vo.ContractPositionStatisticSyncDayVo {
	return historyDomain.days
}

// IsDayComplete says whether a day already holding this many statistics is whole, and
// so is not worth asking the archive about. A whole day is one statistic every five
// minutes, from midnight to the last five minutes before the next.
func (historyDomain ContractPositionStatisticHistoryDomain) IsDayComplete(heldCount int) bool {
	return heldCount >= int(contractPositionStatisticDay/ContractPositionStatisticInterval)
}

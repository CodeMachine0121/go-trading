package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// contractPositionStatisticDay is also the span of one venue archive file.
const contractPositionStatisticDay = 24 * time.Hour

// ContractPositionStatisticHistoryDomain cuts a history sync's lookback into UTC calendar days, matching how the venue files its archive.
// Today and yesterday are walked even though they usually have no file yet, rather than guessing the venue's publishing schedule.
type ContractPositionStatisticHistoryDomain struct {
	days []vo.ContractPositionStatisticSyncDayVo
}

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

// Days are oldest first.
func (historyDomain ContractPositionStatisticHistoryDomain) Days() []vo.ContractPositionStatisticSyncDayVo {
	return historyDomain.days
}

// IsDayComplete reports whether a day already holds one statistic per five minutes, so the archive need not be asked.
func (historyDomain ContractPositionStatisticHistoryDomain) IsDayComplete(heldCount int) bool {
	return heldCount >= int(contractPositionStatisticDay/ContractPositionStatisticInterval)
}

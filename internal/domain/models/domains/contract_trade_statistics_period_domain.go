package domains

import (
	"fmt"
	"strings"
	"time"
)

const contractTradeStatisticsDefaultPeriod = "30d"

var contractTradeStatisticsPeriodDays = map[string]int{"7d": 7, "30d": 30, "90d": 90, "all": 0}

// ContractTradeStatisticsPeriodDomain is how far back to look; all means no limit.
type ContractTradeStatisticsPeriodDomain struct {
	value string
}

// NewContractTradeStatisticsPeriodDomain takes a blank period as the last thirty days.
func NewContractTradeStatisticsPeriodDomain(period string) (ContractTradeStatisticsPeriodDomain, error) {
	value := strings.TrimSpace(period)
	if value == "" {
		value = contractTradeStatisticsDefaultPeriod
	}

	if _, isKnown := contractTradeStatisticsPeriodDays[value]; !isKnown {
		return ContractTradeStatisticsPeriodDomain{}, fmt.Errorf(
			"%w: 期間只有 7d、30d、90d 與 all", ErrContractTradeValidation)
	}

	return ContractTradeStatisticsPeriodDomain{value: value}, nil
}

func (periodDomain ContractTradeStatisticsPeriodDomain) Value() string {
	return periodDomain.value
}

// Since is nil for all.
func (periodDomain ContractTradeStatisticsPeriodDomain) Since(now time.Time) *time.Time {
	days := contractTradeStatisticsPeriodDays[periodDomain.value]
	if days == 0 {
		return nil
	}

	since := now.UTC().AddDate(0, 0, -days)

	return &since
}

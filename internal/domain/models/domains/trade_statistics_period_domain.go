package domains

import (
	"fmt"
	"strings"
	"time"
)

const tradeStatisticsDefaultPeriod = "30d"

var tradeStatisticsPeriodDays = map[string]int{"7d": 7, "30d": 30, "90d": 90, "all": 0}

// TradeStatisticsPeriodDomain is how far back to look; all means no limit.
type TradeStatisticsPeriodDomain struct {
	value string
}

// NewTradeStatisticsPeriodDomain takes a blank period as the last thirty days and refuses an unknown one with the journal's own validation error.
func NewTradeStatisticsPeriodDomain(period string, validationError error) (TradeStatisticsPeriodDomain, error) {
	value := strings.TrimSpace(period)
	if value == "" {
		value = tradeStatisticsDefaultPeriod
	}

	if _, isKnown := tradeStatisticsPeriodDays[value]; !isKnown {
		return TradeStatisticsPeriodDomain{}, fmt.Errorf(
			"%w: 期間只有 7d、30d、90d 與 all", validationError)
	}

	return TradeStatisticsPeriodDomain{value: value}, nil
}

func (periodDomain TradeStatisticsPeriodDomain) Value() string {
	return periodDomain.value
}

// Since is nil for all.
func (periodDomain TradeStatisticsPeriodDomain) Since(now time.Time) *time.Time {
	days := tradeStatisticsPeriodDays[periodDomain.value]
	if days == 0 {
		return nil
	}

	since := now.UTC().AddDate(0, 0, -days)

	return &since
}

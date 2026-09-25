package domains

import "time"

// DailyUsageAllowanceDomain is the assistant's per-UTC-day usage ceiling; usage is passed in because the day it is summed over is named here.
type DailyUsageAllowanceDomain struct {
	allowance int
	now       time.Time
}

func NewDailyUsageAllowanceDomain(allowance int, now time.Time) DailyUsageAllowanceDomain {
	return DailyUsageAllowanceDomain{allowance: allowance, now: now.UTC()}
}

// StartOfDay is the UTC midnight usage is summed from.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) StartOfDay() time.Time {
	universalNow := dailyUsageAllowanceDomain.now

	return time.Date(
		universalNow.Year(), universalNow.Month(), universalNow.Day(),
		0, 0, 0, 0, time.UTC)
}

// ResetsAt is when the allowance comes back.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) ResetsAt() time.Time {
	return dailyUsageAllowanceDomain.StartOfDay().AddDate(0, 0, 1)
}

// Exhausted treats reaching the allowance exactly as spent.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) Exhausted(usageToday int) bool {
	return usageToday >= dailyUsageAllowanceDomain.allowance
}

func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) Allowance() int {
	return dailyUsageAllowanceDomain.allowance
}

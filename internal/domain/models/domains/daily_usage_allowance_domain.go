package domains

import "time"

// DailyUsageAllowanceDomain is today's ceiling on the assistant: where today starts,
// when the allowance comes back, and whether it is already spent.
//
// A day is a universal-time calendar day, the same basis every other time in this
// system uses. The alternative — the reader's own day — would make the allowance come
// back at an hour that depends on who is asking.
//
// Whether it is spent is asked with the day's usage rather than held, because the
// usage has to be summed over the stretch this model is the one to name. Holding it
// would mean knowing the total before knowing which hours to total.
type DailyUsageAllowanceDomain struct {
	allowance int
	now       time.Time
}

// NewDailyUsageAllowanceDomain pins the allowance to the day the given moment falls
// in.
func NewDailyUsageAllowanceDomain(allowance int, now time.Time) DailyUsageAllowanceDomain {
	return DailyUsageAllowanceDomain{allowance: allowance, now: now.UTC()}
}

// StartOfDay is the midnight this day began at. Usage is summed from here.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) StartOfDay() time.Time {
	universalNow := dailyUsageAllowanceDomain.now

	return time.Date(
		universalNow.Year(), universalNow.Month(), universalNow.Day(),
		0, 0, 0, 0, time.UTC)
}

// ResetsAt is the midnight this day ends at, which is when the allowance comes back.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) ResetsAt() time.Time {
	return dailyUsageAllowanceDomain.StartOfDay().AddDate(0, 0, 1)
}

// Exhausted says today's allowance is spent. Reaching it exactly counts as spent:
// the allowance is what may be used, not what may be exceeded.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) Exhausted(usageToday int) bool {
	return usageToday >= dailyUsageAllowanceDomain.allowance
}

// Allowance is the ceiling itself, so that a refusal can name the number it hit.
func (dailyUsageAllowanceDomain DailyUsageAllowanceDomain) Allowance() int {
	return dailyUsageAllowanceDomain.allowance
}

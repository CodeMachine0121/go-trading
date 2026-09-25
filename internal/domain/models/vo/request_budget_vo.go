package vo

// RequestBudgetVo is a refilling allowance: RequestsPerMinute trickle back in, and at most Burst can be saved up.
type RequestBudgetVo struct {
	RequestsPerMinute int
	// Burst lets a page ask for several things at once without being refused.
	Burst int
}

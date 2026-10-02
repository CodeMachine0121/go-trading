package domains

import "time"

// JobLeadershipTermDomain is how long being on duty lasts and how early a replica stops trusting it.
// A replica lets go safetyMargin before the stored lease ends so that, at handover, the old one has already stopped when the new one starts.
type JobLeadershipTermDomain struct {
	leaseDuration time.Duration
	safetyMargin  time.Duration
}

// NewJobLeadershipTermDomain clamps a margin that would leave no time on duty to half the lease, and a negative one to zero.
func NewJobLeadershipTermDomain(leaseDuration time.Duration, safetyMargin time.Duration) JobLeadershipTermDomain {
	if safetyMargin >= leaseDuration {
		safetyMargin = leaseDuration / 2
	}
	if safetyMargin < 0 {
		safetyMargin = 0
	}

	return JobLeadershipTermDomain{leaseDuration: leaseDuration, safetyMargin: safetyMargin}
}

// ExpiresAt is when the stored lease ends if it is not renewed.
func (jobLeadershipTermDomain JobLeadershipTermDomain) ExpiresAt(acquiredAt time.Time) time.Time {
	return acquiredAt.Add(jobLeadershipTermDomain.leaseDuration)
}

// HeldUntil is when this replica stops acting as on duty; the moment itself already counts as not held.
func (jobLeadershipTermDomain JobLeadershipTermDomain) HeldUntil(acquiredAt time.Time) time.Time {
	return acquiredAt.Add(jobLeadershipTermDomain.leaseDuration - jobLeadershipTermDomain.safetyMargin)
}

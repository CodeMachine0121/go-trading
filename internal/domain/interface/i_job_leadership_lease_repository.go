package _interface

import (
	"context"
	"time"
)

//go:generate go tool mockgen -source=i_job_leadership_lease_repository.go -destination=mocks/mock_i_job_leadership_lease_repository.go -package=mocks

// IJobLeadershipLeaseRepository decides who is on duty in one conditional write, so two replicas can never both believe they won.
type IJobLeadershipLeaseRepository interface {
	// Acquire takes or extends the lease to expiresAt when nobody holds it, it has expired at now, or holderName already holds it; false means someone else does.
	Acquire(
		executionContext context.Context, name string, holderName string, now time.Time, expiresAt time.Time,
	) (bool, error)

	// Release gives the lease up at once, but only when holderName still holds it.
	Release(executionContext context.Context, name string, holderName string) error
}

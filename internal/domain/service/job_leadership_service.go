package service

import (
	"context"
	"sync"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// jobLeadershipLeaseName names the single duty every replica competes for.
const jobLeadershipLeaseName = "background-jobs"

// JobLeadershipService remembers whether this replica is on duty, so asking costs no storage read.
type JobLeadershipService struct {
	jobLeadershipLeaseRepository domaininterface.IJobLeadershipLeaseRepository
	clockProxy                   domaininterface.IClockProxy
	term                         domains.JobLeadershipTermDomain
	replicaName                  string

	mutex     sync.Mutex
	held      bool
	heldUntil time.Time
}

func NewJobLeadershipService(
	jobLeadershipLeaseRepository domaininterface.IJobLeadershipLeaseRepository,
	clockProxy domaininterface.IClockProxy,
	term domains.JobLeadershipTermDomain,
	replicaName string,
) *JobLeadershipService {
	return &JobLeadershipService{
		jobLeadershipLeaseRepository: jobLeadershipLeaseRepository,
		clockProxy:                   clockProxy,
		term:                         term,
		replicaName:                  replicaName,
	}
}

// Renew takes or extends the duty; the time is read before the write so a slow write only shortens how long this replica trusts it.
// A failed write counts as not renewed: the duty is kept only until the last successful renewal runs out, never extended.
func (jobLeadershipService *JobLeadershipService) Renew(
	executionContext context.Context,
) (dto.JobLeadershipChangeDto, error) {
	acquiredAt := jobLeadershipService.clockProxy.Now()

	acquired, acquireError := jobLeadershipService.jobLeadershipLeaseRepository.Acquire(
		executionContext, jobLeadershipLeaseName, jobLeadershipService.replicaName,
		acquiredAt, jobLeadershipService.term.ExpiresAt(acquiredAt))

	jobLeadershipService.mutex.Lock()
	defer jobLeadershipService.mutex.Unlock()

	if acquireError != nil {
		// Reported as lost only once the last successful renewal has run out.
		ranOut := jobLeadershipService.held && !acquiredAt.Before(jobLeadershipService.heldUntil)
		if ranOut {
			jobLeadershipService.held = false
		}

		return dto.JobLeadershipChangeDto{Lost: ranOut}, acquireError
	}

	wasHeld := jobLeadershipService.held
	jobLeadershipService.held = acquired
	jobLeadershipService.heldUntil = jobLeadershipService.term.HeldUntil(acquiredAt)

	change := dto.JobLeadershipChangeDto{
		Gained: !wasHeld && jobLeadershipService.held,
		Lost:   wasHeld && !jobLeadershipService.held,
	}

	return change, nil
}

// IsLeader reads the clock on every question, held or not, so each question is a moment the answer can be checked against.
func (jobLeadershipService *JobLeadershipService) IsLeader() bool {
	now := jobLeadershipService.clockProxy.Now()

	jobLeadershipService.mutex.Lock()
	defer jobLeadershipService.mutex.Unlock()

	return jobLeadershipService.held && now.Before(jobLeadershipService.heldUntil)
}

// Release gives the duty back so the next replica need not wait out the lease, and reports whether there was one to give.
func (jobLeadershipService *JobLeadershipService) Release(executionContext context.Context) (bool, error) {
	if !jobLeadershipService.forgetDuty() {
		return false, nil
	}

	return true, jobLeadershipService.jobLeadershipLeaseRepository.Release(
		executionContext, jobLeadershipLeaseName, jobLeadershipService.replicaName)
}

// forgetDuty exists to scope the lock with defer, so the storage write in Release happens after the lock is let go.
func (jobLeadershipService *JobLeadershipService) forgetDuty() bool {
	jobLeadershipService.mutex.Lock()
	defer jobLeadershipService.mutex.Unlock()

	wasHeld := jobLeadershipService.held
	jobLeadershipService.held = false

	return wasHeld
}

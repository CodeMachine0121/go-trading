package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// JobLeadershipApplication is all a background job needs to know about replicas: whether this one is on duty.
type JobLeadershipApplication struct {
	jobLeadershipService *service.JobLeadershipService
}

func NewJobLeadershipApplication(jobLeadershipService *service.JobLeadershipService) *JobLeadershipApplication {
	return &JobLeadershipApplication{jobLeadershipService: jobLeadershipService}
}

func (jobLeadershipApplication *JobLeadershipApplication) RenewLeadership(
	executionContext context.Context,
) (dto.JobLeadershipChangeDto, error) {
	return jobLeadershipApplication.jobLeadershipService.Renew(executionContext)
}

// IsLeader reads memory only, so a job can ask at the top of every round.
func (jobLeadershipApplication *JobLeadershipApplication) IsLeader() bool {
	return jobLeadershipApplication.jobLeadershipService.IsLeader()
}

func (jobLeadershipApplication *JobLeadershipApplication) ReleaseLeadership(executionContext context.Context) error {
	return jobLeadershipApplication.jobLeadershipService.Release(executionContext)
}

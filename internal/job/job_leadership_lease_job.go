package job

import (
	"context"
	"log"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
)

// JobLeadershipLeaseJob keeps this replica's claim on duty fresh; it never gives the duty back on Stop,
// because rounds still in flight would then overlap the next replica's — the server releases it after they end.
type JobLeadershipLeaseJob struct {
	*repeatingRound
}

func NewJobLeadershipLeaseJob(
	jobLeadershipApplication *application.JobLeadershipApplication, interval time.Duration,
) *JobLeadershipLeaseJob {
	return &JobLeadershipLeaseJob{repeatingRound: newRepeatingRound(interval,
		func(executionContext context.Context) {
			change, renewError := jobLeadershipApplication.RenewLeadership(executionContext)
			if renewError != nil {
				log.Printf("job leadership could not be renewed: %v", renewError)
			}
			if change.Gained {
				log.Printf("job leadership gained: this replica now runs the once-per-system jobs")
			}
			if change.Lost {
				log.Printf("job leadership lost: this replica stops the once-per-system jobs")
			}
		})}
}

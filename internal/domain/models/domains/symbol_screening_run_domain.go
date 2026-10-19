package domains

import (
	"errors"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

const (
	SymbolScreeningInterruptedReason   = "被重新啟動打斷"
	symbolScreeningOverAllowanceReason = "超過時間上限"
	symbolScreeningBrokeDownReason     = "篩選服務壞掉了，這筆沒有跑完"
)

type SymbolScreeningRunDomain struct {
	run    entities.SymbolScreeningRun
	status vo.SymbolScreeningRunStatusVo
}

func NewSymbolScreeningRunDomain(run entities.SymbolScreeningRun) SymbolScreeningRunDomain {
	return SymbolScreeningRunDomain{run: run, status: vo.NewSymbolScreeningRunStatusVo(run.Status)}
}

// NewSubmittedSymbolScreeningRunDomain queues the run with every symbol waiting in the order it was pasted.
func NewSubmittedSymbolScreeningRunDomain(
	ownerID uint,
	profileID uint,
	snapshot dto.SymbolScreeningSnapshotDto,
	symbols []string,
	submittedAt time.Time,
) SymbolScreeningRunDomain {
	candidates := make([]entities.SymbolScreeningCandidate, 0, len(symbols))
	for index, symbol := range symbols {
		candidates = append(candidates, entities.SymbolScreeningCandidate{
			Position: index + 1,
			Symbol:   symbol,
			Status:   string(vo.SymbolScreeningCandidateWaiting),
		})
	}

	return NewSymbolScreeningRunDomain(entities.SymbolScreeningRun{
		OwnerID:        ownerID,
		ProfileID:      &profileID,
		ProfileName:    snapshot.ProfileName,
		Snapshot:       snapshot,
		Status:         string(vo.SymbolScreeningRunQueued),
		CandidateCount: len(symbols),
		SubmittedAt:    submittedAt.UTC(),
		Candidates:     candidates,
	})
}

func (runDomain SymbolScreeningRunDomain) IsActive() bool {
	return runDomain.status == vo.SymbolScreeningRunQueued || runDomain.status == vo.SymbolScreeningRunRunning
}

func (runDomain SymbolScreeningRunDomain) Snapshot() dto.SymbolScreeningSnapshotDto {
	return runDomain.run.Snapshot
}

func (runDomain SymbolScreeningRunDomain) ProgressPercent() int {
	if runDomain.status == vo.SymbolScreeningRunSucceeded {
		return 100
	}
	if runDomain.run.CandidateCount <= 0 {
		return 0
	}

	return min(100, runDomain.run.ProcessedCandidateCount*100/runDomain.run.CandidateCount)
}

func (runDomain SymbolScreeningRunDomain) Claimed(replicaName string, startedAt time.Time) SymbolScreeningRunDomain {
	startedAtUtc := startedAt.UTC()
	runDomain.run.Status = string(vo.SymbolScreeningRunRunning)
	runDomain.run.ReplicaName = replicaName
	runDomain.run.StartedAt = &startedAtUtc
	runDomain.status = vo.SymbolScreeningRunRunning

	return runDomain
}

func (runDomain SymbolScreeningRunDomain) Advanced(processedCandidateCount int) SymbolScreeningRunDomain {
	runDomain.run.ProcessedCandidateCount = processedCandidateCount

	return runDomain
}

func (runDomain SymbolScreeningRunDomain) Succeeded(
	contributionFailureReason string, finishedAt time.Time,
) SymbolScreeningRunDomain {
	finishedAtUtc := finishedAt.UTC()
	runDomain.run.Status = string(vo.SymbolScreeningRunSucceeded)
	runDomain.run.ProcessedCandidateCount = runDomain.run.CandidateCount
	runDomain.run.ContributionFailureReason = contributionFailureReason
	runDomain.run.FinishedAt = &finishedAtUtc
	runDomain.status = vo.SymbolScreeningRunSucceeded

	return runDomain
}

// FailedFor words why the run stopped: a restart says so whatever the screening reported.
func (runDomain SymbolScreeningRunDomain) FailedFor(
	screeningError error, interrupted bool, finishedAt time.Time,
) SymbolScreeningRunDomain {
	reason := symbolScreeningBrokeDownReason
	switch {
	case interrupted:
		reason = SymbolScreeningInterruptedReason
	case errors.Is(screeningError, ErrBacktestTimeAllowanceSpent):
		reason = symbolScreeningOverAllowanceReason
	case screeningError != nil && !errors.Is(screeningError, ErrSymbolScreeningBrokeDown):
		reason = screeningError.Error()
	}

	finishedAtUtc := finishedAt.UTC()
	runDomain.run.Status = string(vo.SymbolScreeningRunFailed)
	runDomain.run.FailureReason = reason
	runDomain.run.FinishedAt = &finishedAtUtc
	runDomain.status = vo.SymbolScreeningRunFailed

	return runDomain
}

func (runDomain SymbolScreeningRunDomain) Cancelled(cancelledAt time.Time) (SymbolScreeningRunDomain, error) {
	if !runDomain.IsActive() {
		return SymbolScreeningRunDomain{}, ErrSymbolScreeningRunAlreadyEnded
	}

	cancelledAtUtc := cancelledAt.UTC()
	runDomain.run.Status = string(vo.SymbolScreeningRunCancelled)
	runDomain.run.FinishedAt = &cancelledAtUtc
	runDomain.status = vo.SymbolScreeningRunCancelled

	return runDomain, nil
}

func (runDomain SymbolScreeningRunDomain) ToEntity() entities.SymbolScreeningRun {
	return runDomain.run
}

func (runDomain SymbolScreeningRunDomain) ToDto() dto.SymbolScreeningRunDto {
	run := runDomain.run
	startedAt := (*time.Time)(nil)
	if run.StartedAt != nil {
		startedAtUtc := run.StartedAt.UTC()
		startedAt = &startedAtUtc
	}
	finishedAt := (*time.Time)(nil)
	if run.FinishedAt != nil {
		finishedAtUtc := run.FinishedAt.UTC()
		finishedAt = &finishedAtUtc
	}

	return dto.SymbolScreeningRunDto{
		ID:                        run.ID,
		ProfileID:                 run.ProfileID,
		ProfileName:               run.ProfileName,
		Status:                    string(runDomain.status),
		ProcessedCandidateCount:   run.ProcessedCandidateCount,
		CandidateCount:            run.CandidateCount,
		ProgressPercent:           runDomain.ProgressPercent(),
		FailureReason:             run.FailureReason,
		ContributionFailureReason: run.ContributionFailureReason,
		SubmittedAt:               run.SubmittedAt.UTC(),
		StartedAt:                 startedAt,
		FinishedAt:                finishedAt,
	}
}

// ToDetailDto lists ranked candidates first by rank, the rest by the order they were pasted.
func (runDomain SymbolScreeningRunDomain) ToDetailDto(
	candidates []entities.SymbolScreeningCandidate,
) dto.SymbolScreeningRunDetailDto {
	ordered := NewSymbolScreeningRecommendationDomain(candidates).InDisplayOrder()
	candidateDtos := make([]dto.SymbolScreeningCandidateDto, 0, len(ordered))
	for _, candidate := range ordered {
		candidateDtos = append(candidateDtos, candidate.ToDto())
	}

	return dto.SymbolScreeningRunDetailDto{
		SymbolScreeningRunDto: runDomain.ToDto(),
		Snapshot:              runDomain.run.Snapshot,
		Candidates:            candidateDtos,
	}
}

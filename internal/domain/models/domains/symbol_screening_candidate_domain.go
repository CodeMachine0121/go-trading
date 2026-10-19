package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SymbolScreeningCandidateDomain moves one candidate through its verdicts, so a stage is only ever set together with its reason.
type SymbolScreeningCandidateDomain struct {
	candidate entities.SymbolScreeningCandidate
}

func NewSymbolScreeningCandidateDomain(candidate entities.SymbolScreeningCandidate) SymbolScreeningCandidateDomain {
	return SymbolScreeningCandidateDomain{candidate: candidate}
}

func (candidateDomain SymbolScreeningCandidateDomain) Processing() SymbolScreeningCandidateDomain {
	candidateDomain.candidate.Status = string(vo.SymbolScreeningCandidateProcessing)

	return candidateDomain
}

func (candidateDomain SymbolScreeningCandidateDomain) Passed(findings dto.SymbolScreeningFindingsDto) SymbolScreeningCandidateDomain {
	return candidateDomain.endedAs(vo.SymbolScreeningCandidatePassed, vo.SymbolScreeningStageNone, "", findings)
}

func (candidateDomain SymbolScreeningCandidateDomain) Rejected(
	stage vo.SymbolScreeningStageVo, reason string, findings dto.SymbolScreeningFindingsDto,
) SymbolScreeningCandidateDomain {
	return candidateDomain.endedAs(vo.SymbolScreeningCandidateRejected, stage, reason, findings)
}

// Unfinished keeps whatever figures were gathered before the failure.
func (candidateDomain SymbolScreeningCandidateDomain) Unfinished(
	reason string, findings dto.SymbolScreeningFindingsDto,
) SymbolScreeningCandidateDomain {
	return candidateDomain.endedAs(vo.SymbolScreeningCandidateUnfinished, vo.SymbolScreeningStageNone, reason, findings)
}

func (candidateDomain SymbolScreeningCandidateDomain) endedAs(
	status vo.SymbolScreeningCandidateStatusVo, stage vo.SymbolScreeningStageVo, reason string,
	findings dto.SymbolScreeningFindingsDto,
) SymbolScreeningCandidateDomain {
	candidateDomain.candidate.Status = string(status)
	candidateDomain.candidate.Stage = string(stage)
	candidateDomain.candidate.Reason = reason
	candidateDomain.candidate.Findings = findings

	return candidateDomain
}

func (candidateDomain SymbolScreeningCandidateDomain) ToEntity() entities.SymbolScreeningCandidate {
	return candidateDomain.candidate
}

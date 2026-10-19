package vo

type SymbolScreeningCandidateStatusVo string

const (
	SymbolScreeningCandidateWaiting    SymbolScreeningCandidateStatusVo = "waiting"
	SymbolScreeningCandidateProcessing SymbolScreeningCandidateStatusVo = "processing"
	SymbolScreeningCandidatePassed     SymbolScreeningCandidateStatusVo = "passed"
	SymbolScreeningCandidateRejected   SymbolScreeningCandidateStatusVo = "rejected"
	SymbolScreeningCandidateUnfinished SymbolScreeningCandidateStatusVo = "unfinished"
	SymbolScreeningCandidateCancelled  SymbolScreeningCandidateStatusVo = "cancelled"
)

// NewSymbolScreeningCandidateStatusVo reads anything unrecognised as unfinished.
func NewSymbolScreeningCandidateStatusVo(status string) SymbolScreeningCandidateStatusVo {
	switch SymbolScreeningCandidateStatusVo(status) {
	case SymbolScreeningCandidateWaiting, SymbolScreeningCandidateProcessing, SymbolScreeningCandidatePassed,
		SymbolScreeningCandidateRejected, SymbolScreeningCandidateCancelled:
		return SymbolScreeningCandidateStatusVo(status)
	default:
		return SymbolScreeningCandidateUnfinished
	}
}

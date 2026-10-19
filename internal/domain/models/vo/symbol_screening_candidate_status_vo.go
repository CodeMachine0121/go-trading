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

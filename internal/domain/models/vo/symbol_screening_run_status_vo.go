package vo

type SymbolScreeningRunStatusVo string

const (
	SymbolScreeningRunQueued    SymbolScreeningRunStatusVo = "queued"
	SymbolScreeningRunRunning   SymbolScreeningRunStatusVo = "running"
	SymbolScreeningRunSucceeded SymbolScreeningRunStatusVo = "succeeded"
	SymbolScreeningRunFailed    SymbolScreeningRunStatusVo = "failed"
	SymbolScreeningRunCancelled SymbolScreeningRunStatusVo = "cancelled"
)

// NewSymbolScreeningRunStatusVo reads anything unrecognised as failed, whose worst case is screening again.
func NewSymbolScreeningRunStatusVo(status string) SymbolScreeningRunStatusVo {
	switch SymbolScreeningRunStatusVo(status) {
	case SymbolScreeningRunQueued, SymbolScreeningRunRunning, SymbolScreeningRunSucceeded, SymbolScreeningRunCancelled:
		return SymbolScreeningRunStatusVo(status)
	default:
		return SymbolScreeningRunFailed
	}
}

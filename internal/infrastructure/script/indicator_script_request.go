package script

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

// indicatorScriptRequestHeader is the first thing a compartment reads, before it knows
// what kind of input follows. It carries what the compartment needs before it can read
// anything else: which kind of market the input comes from, since that decides how the
// rest is read; the memory cap, which must be in place before any of the script is
// looked at; and the allowance, which the sandbox is built with.
type indicatorScriptRequestHeader struct {
	MarketKind       string
	MemoryLimitBytes int64
	ExecutionTimeout time.Duration
}

// indicatorScriptRequest is one run, handed across to the compartment in full. Every
// field is plain data: the service's own models do not cross the process boundary,
// and the compartment rebuilds the ones it needs from what arrives here.
type indicatorScriptRequest[Input any] struct {
	Script     string
	ResultType string
	// Parameters are the knobs with this run's values already applied, so the
	// compartment reads exactly what the caller settled on.
	Parameters []dto.StrategyScriptParameterDto
	// ForEachElement asks for a replay — one run per element over a growing stretch —
	// rather than a single run over the whole input.
	ForEachElement bool
	Input          []Input
}

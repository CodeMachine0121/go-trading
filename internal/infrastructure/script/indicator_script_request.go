package script

import (
	"encoding/gob"
	"fmt"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptRequestHeader is read first and carries the market kind, memory cap and allowance needed before the rest of the request.
type indicatorScriptRequestHeader struct {
	MarketKind       string
	MemoryLimitBytes int64
	ExecutionTimeout time.Duration
}

// answer serves the request inside the compartment, setting the OS memory cap before the script is read.
func (header indicatorScriptRequestHeader) answer(
	headerError error, decoder *gob.Decoder,
) indicatorScriptResponse {
	if headerError != nil {
		return newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：算式隔間讀不到要算的內容：%v", domains.ErrIndicatorScriptFailed, headerError))
	}

	if header.MemoryLimitBytes > 0 {
		// A soft GC limit at 80% of the cap slows memory churn before the hard limit is hit.
		debug.SetMemoryLimit(header.MemoryLimitBytes / 10 * 8)

		// RLIMIT_DATA is used instead of an address-space limit because the Go runtime reserves far more address space than it uses.
		limitError := syscall.Setrlimit(syscall.RLIMIT_DATA, &syscall.Rlimit{
			Cur: uint64(header.MemoryLimitBytes),
			Max: uint64(header.MemoryLimitBytes),
		})
		// The cap is only enforced on Linux (the production OS); elsewhere a refused limit does not block the script.
		if limitError != nil && runtime.GOOS == "linux" {
			return newFailedIndicatorScriptResponse(fmt.Errorf(
				"%w: 算式執行失敗：算式隔間無法設定記憶體上限：%v", domains.ErrIndicatorScriptFailed, limitError))
		}
	}

	switch header.MarketKind {
	case spotIndicatorScriptInput.marketKind:
		return indicatorScriptRunner[vo.KCandleVo]{
			executionTimeout: header.ExecutionTimeout,
			input:            spotIndicatorScriptInput,
		}.answer(decoder)
	case contractIndicatorScriptInput.marketKind:
		return indicatorScriptRunner[vo.ContractKCandleVo]{
			executionTimeout: header.ExecutionTimeout,
			input:            contractIndicatorScriptInput,
		}.answer(decoder)
	default:
		return newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：認不得的行情種類 %q", domains.ErrIndicatorScriptFailed, header.MarketKind))
	}
}

// indicatorScriptRequest carries only plain data across the process boundary.
type indicatorScriptRequest[Input any] struct {
	Script     string
	ResultType string
	// Parameters already have this run's values applied.
	Parameters     []dto.StrategyScriptParameterDto
	ForEachElement bool
	Input          []Input
}

package script

import (
	"encoding/gob"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// indicatorScriptRequestHeader is read first and carries the market kind, limits and allowance needed before the rest of the request.
type indicatorScriptRequestHeader struct {
	MarketKind       string
	MemoryLimitBytes int64
	ExecutionTimeout time.Duration
	// ProcessorTimeLimit backs up the parent's own timer in case it fails to stop the compartment.
	ProcessorTimeLimit time.Duration
}

// answer serves the request inside the compartment, setting the OS memory cap before the script is read.
func (header indicatorScriptRequestHeader) answer(
	headerError error, decoder *gob.Decoder,
) indicatorScriptResponse {
	if headerError != nil {
		return newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：算式隔間讀不到要算的內容：%v", domains.ErrIndicatorScriptFailed, headerError))
	}

	if header.ProcessorTimeLimit > 0 {
		// One processor keeps processor time within wall time, so the limit cannot cut short a script its allowance would let finish.
		runtime.GOMAXPROCS(1)
		processorSeconds := uint64((header.ProcessorTimeLimit + time.Second - 1) / time.Second)
		// The runtime ignores SIGXCPU, so the soft limit is acted on here; the hard limit is the kernel's backstop.
		processorTimeSpent := make(chan os.Signal, 1)
		signal.Notify(processorTimeSpent, syscall.SIGXCPU)
		go func() {
			<-processorTimeSpent
			os.Exit(1)
		}()
		limitError := syscall.Setrlimit(syscall.RLIMIT_CPU, &syscall.Rlimit{
			Cur: processorSeconds,
			Max: processorSeconds + 1,
		})
		if limitError != nil && runtime.GOOS == "linux" {
			return newFailedIndicatorScriptResponse(fmt.Errorf(
				"%w: 算式執行失敗：算式隔間無法設定處理器時間上限：%v", domains.ErrIndicatorScriptFailed, limitError))
		}
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

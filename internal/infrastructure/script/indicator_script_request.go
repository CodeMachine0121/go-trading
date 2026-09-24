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

// answer serves the request this header announces, inside the compartment. Everything
// that makes the compartment safe to fail happens here, before the script is looked
// at: the memory cap is set on this process by the operating system, so a script that
// reaches past it ends this process — and only this process. headerError is how
// reading this header went; a header that never arrived is answered as such.
func (header indicatorScriptRequestHeader) answer(
	headerError error, decoder *gob.Decoder,
) indicatorScriptResponse {
	if headerError != nil {
		return newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：算式隔間讀不到要算的內容：%v", domains.ErrIndicatorScriptFailed, headerError))
	}

	if header.MemoryLimitBytes > 0 {
		// The collector is told to work hard well before the wall, so a script that
		// merely churns through memory is slowed rather than stopped. Only one that
		// genuinely holds more than the cap runs into it.
		debug.SetMemoryLimit(header.MemoryLimitBytes / 10 * 8)

		// The data limit counts only memory that can be written to. The Go runtime
		// reserves a great deal of address space it never touches, and a limit on
		// address space would trip over that reservation long before the script had
		// used anything; the data limit trips only when the heap actually grows.
		limitError := syscall.Setrlimit(syscall.RLIMIT_DATA, &syscall.Rlimit{
			Cur: uint64(header.MemoryLimitBytes),
			Max: uint64(header.MemoryLimitBytes),
		})
		// Only Linux is promised the cap — that is what the service runs on. Anywhere
		// else the compartment still keeps the service apart from the script, and a
		// refused limit is no reason to refuse the script. On Linux, a compartment
		// that cannot cap itself does not run the script at all.
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

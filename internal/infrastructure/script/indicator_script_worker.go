package script

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"syscall"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// IndicatorScriptWorkerCommand is the argument that turns the server binary into a
// script compartment instead of a server. The service starts itself with it for every
// run of a script.
const IndicatorScriptWorkerCommand = "indicator-script-worker"

// IndicatorScriptWorker is the inside of a script compartment: it serves exactly one
// request, in a process of its own, and then that process ends.
//
// Everything that makes the compartment safe to fail happens here, before the script
// is looked at: the memory cap is set on this process by the operating system, so a
// script that reaches past it ends this process — and only this process.
type IndicatorScriptWorker struct{}

func NewIndicatorScriptWorker() *IndicatorScriptWorker {
	return &IndicatorScriptWorker{}
}

// Serve reads one request from input, runs it, and writes the answer to output. It
// returns the exit code the process should end with: zero whenever an answer was
// written, however the script itself fared.
func (indicatorScriptWorker *IndicatorScriptWorker) Serve(input io.Reader, output io.Writer) int {
	decoder := gob.NewDecoder(input)

	var header indicatorScriptRequestHeader
	if decodeError := decoder.Decode(&header); decodeError != nil {
		return indicatorScriptWorker.reply(output, newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：算式隔間讀不到要算的內容：%v", domains.ErrIndicatorScriptFailed, decodeError)))
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
			return indicatorScriptWorker.reply(output, newFailedIndicatorScriptResponse(fmt.Errorf(
				"%w: 算式執行失敗：算式隔間無法設定記憶體上限：%v", domains.ErrIndicatorScriptFailed, limitError)))
		}
	}

	switch header.MarketKind {
	case spotIndicatorScriptInput.marketKind:
		return indicatorScriptWorker.reply(output, indicatorScriptRunner[vo.KCandleVo]{
			executionTimeout: header.ExecutionTimeout,
			input:            spotIndicatorScriptInput,
		}.answer(decoder))
	case contractIndicatorScriptInput.marketKind:
		return indicatorScriptWorker.reply(output, indicatorScriptRunner[vo.ContractKCandleVo]{
			executionTimeout: header.ExecutionTimeout,
			input:            contractIndicatorScriptInput,
		}.answer(decoder))
	default:
		return indicatorScriptWorker.reply(output, newFailedIndicatorScriptResponse(fmt.Errorf(
			"%w: 算式執行失敗：認不得的行情種類 %q", domains.ErrIndicatorScriptFailed, header.MarketKind)))
	}
}

// reply writes the one answer this compartment gives. A compartment that cannot even
// write its answer has nothing left to say it with, so it says so on its error output
// and ends with a failing code; the service reads that as the compartment having gone
// down.
func (indicatorScriptWorker *IndicatorScriptWorker) reply(
	output io.Writer, response indicatorScriptResponse,
) int {
	if encodeError := gob.NewEncoder(output).Encode(response); encodeError != nil {
		fmt.Fprintf(os.Stderr, "indicator script worker could not reply: %v\n", encodeError)
		return 1
	}

	return 0
}

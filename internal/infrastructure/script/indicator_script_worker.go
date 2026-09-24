package script

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
)

// IndicatorScriptWorkerCommand is the argument that turns the server binary into a
// script compartment instead of a server. The service starts itself with it for every
// run of a script.
const IndicatorScriptWorkerCommand = "indicator-script-worker"

// IndicatorScriptWorker is the inside of a script compartment: it serves exactly one
// request, in a process of its own, and then that process ends.
//
// What the request header announces decides everything else — the memory cap is set
// and the matching sandbox is built from it before the script is looked at (see
// indicatorScriptRequestHeader.answer).
type IndicatorScriptWorker struct{}

func NewIndicatorScriptWorker() *IndicatorScriptWorker {
	return &IndicatorScriptWorker{}
}

// Serve reads one request from input, runs it, and writes the answer to output. It
// returns the exit code the process should end with: zero whenever an answer was
// written, however the script itself fared. A compartment that cannot even write its
// answer has nothing left to say it with, so it says so on its error output and ends
// with a failing code; the service reads that as the compartment having gone down.
func (indicatorScriptWorker *IndicatorScriptWorker) Serve(input io.Reader, output io.Writer) int {
	decoder := gob.NewDecoder(input)

	var header indicatorScriptRequestHeader
	headerError := decoder.Decode(&header)

	if encodeError := gob.NewEncoder(output).Encode(header.answer(headerError, decoder)); encodeError != nil {
		fmt.Fprintf(os.Stderr, "indicator script worker could not reply: %v\n", encodeError)
		return 1
	}

	return 0
}

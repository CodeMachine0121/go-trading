package script

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
)

// IndicatorScriptWorkerCommand is the argument that makes the server binary act as a script compartment.
const IndicatorScriptWorkerCommand = "indicator-script-worker"

// IndicatorScriptWorker serves exactly one request in its own process, then exits.
type IndicatorScriptWorker struct{}

func NewIndicatorScriptWorker() *IndicatorScriptWorker {
	return &IndicatorScriptWorker{}
}

// Serve returns exit code zero whenever an answer was written, regardless of script outcome; a failure to write exits non-zero.
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

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
	encoder := gob.NewEncoder(output)

	var header indicatorScriptRequestHeader
	headerError := decoder.Decode(&header)

	// Element messages of a replay are written as they are computed; the closing message is the one answer every request ends with.
	closing := header.answer(headerError, decoder, func(element indicatorScriptResponse) error {
		return encoder.Encode(element)
	})
	if encodeError := encoder.Encode(closing); encodeError != nil {
		fmt.Fprintf(os.Stderr, "indicator script worker could not reply: %v\n", encodeError)
		return 1
	}

	return 0
}

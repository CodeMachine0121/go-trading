package script

import "time"

// IndicatorScriptIsolation is how every indicator script is kept apart from the
// service that asked for it: each run happens in a child process of its own, and that
// child caps its own memory before it touches a line of the script. A script that
// eats more than it may then takes down only its compartment — never the service,
// every bot, and everyone else's screen with it.
//
// It is handed in by the composition root rather than worked out here, because the
// one thing that differs between running for real and running under test is which
// program plays the child: the server binary in production, the test binary in tests.
type IndicatorScriptIsolation struct {
	// WorkerCommand is the program and arguments that start a compartment. The
	// program must answer by serving exactly one IndicatorScriptWorker session on its
	// standard input and output.
	WorkerCommand []string
	// WorkerEnvironment is the whole environment a compartment starts with. Nothing
	// is inherited: the service's own environment holds the database password and
	// the signing keys, and a compartment has no use for either.
	WorkerEnvironment []string
	// ExecutionTimeout is the allowance each run of the script gets — per element in
	// a replay, once for a single calculation.
	ExecutionTimeout time.Duration
	// MemoryLimitBytes is the most memory a compartment may take. Zero means no cap,
	// which is only ever right under test.
	MemoryLimitBytes int64
}

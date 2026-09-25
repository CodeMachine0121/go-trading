package script

import "time"

// IndicatorScriptIsolation runs each script in a memory-capped child process; the composition root supplies the worker command (server binary in production, test binary in tests).
type IndicatorScriptIsolation struct {
	// WorkerCommand must serve exactly one IndicatorScriptWorker session over stdin/stdout.
	WorkerCommand []string
	// WorkerEnvironment is the compartment's entire environment; nothing is inherited, keeping database passwords and signing keys out.
	WorkerEnvironment []string
	// ExecutionTimeout applies per element in a replay, or once for a single calculation.
	ExecutionTimeout time.Duration
	// MemoryLimitBytes of zero means no cap, which is only for tests.
	MemoryLimitBytes int64
}

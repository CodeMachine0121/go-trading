package script_test

import (
	"os"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
)

// workerRoleVariable is set on a compartment started from these tests. The test binary
// then plays the part the server binary plays in production: it serves one request
// and ends, instead of running the tests again.
const workerRoleVariable = "GO_TRADING_TEST_INDICATOR_SCRIPT_WORKER"

// testMemoryLimitBytes is the cap the service ships with, so what passes here is what
// passes there.
const testMemoryLimitBytes = 512 << 20

func TestMain(m *testing.M) {
	if os.Getenv(workerRoleVariable) == "1" {
		os.Exit(script.NewIndicatorScriptWorker().Serve(os.Stdin, os.Stdout))
	}

	os.Exit(m.Run())
}

// isolationWith is the compartment every test runs its scripts in: this very test
// binary, capped exactly as the service is.
//
// Under the race detector the compartment is left uncapped. The detector's shadow
// memory counts against the cap several times over, so an ordinary replay can run
// into it and go down for a reason the shipped binary never meets. The cap itself is
// proved in the pipeline's race-free run, where it is the real one.
func isolationWith(executionTimeout time.Duration) script.IndicatorScriptIsolation {
	memoryLimitBytes := int64(testMemoryLimitBytes)
	if raceDetectorOn {
		memoryLimitBytes = 0
	}

	return script.IndicatorScriptIsolation{
		WorkerCommand:     []string{os.Args[0]},
		WorkerEnvironment: []string{workerRoleVariable + "=1"},
		ExecutionTimeout:  executionTimeout,
		MemoryLimitBytes:  memoryLimitBytes,
	}
}

func spotIndicatorScriptProxy(executionTimeout time.Duration) *script.YaegiIndicatorScriptProxy {
	return script.NewYaegiIndicatorScriptProxy(isolationWith(executionTimeout))
}

func contractIndicatorScriptProxy(executionTimeout time.Duration) *script.YaegiContractIndicatorScriptProxy {
	return script.NewYaegiContractIndicatorScriptProxy(isolationWith(executionTimeout))
}

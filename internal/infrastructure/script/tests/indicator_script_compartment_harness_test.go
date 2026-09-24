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
func isolationWith(executionTimeout time.Duration) script.IndicatorScriptIsolation {
	return script.IndicatorScriptIsolation{
		WorkerCommand:     []string{os.Args[0]},
		WorkerEnvironment: []string{workerRoleVariable + "=1"},
		ExecutionTimeout:  executionTimeout,
		MemoryLimitBytes:  testMemoryLimitBytes,
	}
}

func spotIndicatorScriptProxy(executionTimeout time.Duration) *script.YaegiIndicatorScriptProxy {
	return script.NewYaegiIndicatorScriptProxy(isolationWith(executionTimeout))
}

func contractIndicatorScriptProxy(executionTimeout time.Duration) *script.YaegiContractIndicatorScriptProxy {
	return script.NewYaegiContractIndicatorScriptProxy(isolationWith(executionTimeout))
}

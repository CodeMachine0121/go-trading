package script_test

import (
	"os"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/infrastructure/script"
)

// workerRoleVariable makes the test binary act as a compartment worker, as the server binary does in production.
const workerRoleVariable = "GO_TRADING_TEST_INDICATOR_SCRIPT_WORKER"

// testMemoryLimitBytes matches the shipped cap.
const testMemoryLimitBytes = 512 << 20

func TestMain(m *testing.M) {
	if os.Getenv(workerRoleVariable) == "1" {
		os.Exit(script.NewIndicatorScriptWorker().Serve(os.Stdin, os.Stdout))
	}

	os.Exit(m.Run())
}

// isolationWith runs compartments as this test binary with the shipped cap, left uncapped under the race detector whose shadow memory would trip it.
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
		// Roomy enough that only the concurrency tests, which bring their own, ever wait.
		CompartmentSlots: script.NewIndicatorScriptCompartmentSlots(16),
	}
}

func spotIndicatorScriptProxy(executionTimeout time.Duration) *script.YaegiIndicatorScriptProxy {
	return script.NewYaegiIndicatorScriptProxy(isolationWith(executionTimeout))
}

func contractIndicatorScriptProxy(executionTimeout time.Duration) *script.YaegiContractIndicatorScriptProxy {
	return script.NewYaegiContractIndicatorScriptProxy(isolationWith(executionTimeout))
}

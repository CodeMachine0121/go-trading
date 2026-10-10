package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesTheBacktestRunDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Equal(t, config.BacktestRunConfig{
		WorkerCount:            2,
		OwnerActiveLimit:       3,
		TimeAllowance:          30 * time.Minute,
		Retention:              30 * 24 * time.Hour,
		PollInterval:           2 * time.Second,
		ProgressWriteEveryBars: 100,
	}, applicationConfig.BacktestRun)
	assert.Equal(t, 200, applicationConfig.BacktestDefaultWarmupBarCount)
}

func TestLoadReadsTheBacktestRunSettings(t *testing.T) {
	t.Setenv("BACKTEST_RUN_WORKER_COUNT", "1")
	t.Setenv("BACKTEST_RUN_OWNER_ACTIVE_LIMIT", "5")
	t.Setenv("BACKTEST_RUN_TIME_ALLOWANCE_MINUTES", "45")
	t.Setenv("BACKTEST_RUN_RETENTION_DAYS", "7")
	t.Setenv("BACKTEST_RUN_POLL_INTERVAL_SECONDS", "10")
	t.Setenv("BACKTEST_RUN_PROGRESS_WRITE_EVERY_BARS", "500")
	t.Setenv("BACKTEST_DEFAULT_WARMUP_BAR_COUNT", "1000")

	applicationConfig := config.Load()

	assert.Equal(t, config.BacktestRunConfig{
		WorkerCount:            1,
		OwnerActiveLimit:       5,
		TimeAllowance:          45 * time.Minute,
		Retention:              7 * 24 * time.Hour,
		PollInterval:           10 * time.Second,
		ProgressWriteEveryBars: 500,
	}, applicationConfig.BacktestRun)
	assert.Equal(t, 1000, applicationConfig.BacktestDefaultWarmupBarCount)
}

func TestLoadFallsBackToTheDefaultForAnUnusableOwnerLimit(t *testing.T) {
	for _, unusable := range []string{"0", "-1", "abc"} {
		t.Run(unusable, func(t *testing.T) {
			t.Setenv("BACKTEST_RUN_OWNER_ACTIVE_LIMIT", unusable)
			t.Setenv("BACKTEST_RUN_WORKER_COUNT", unusable)

			applicationConfig := config.Load()

			assert.Equal(t, 3, applicationConfig.BacktestRun.OwnerActiveLimit)
			assert.Equal(t, 2, applicationConfig.BacktestRun.WorkerCount)
		})
	}
}

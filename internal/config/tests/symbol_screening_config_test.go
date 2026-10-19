package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesTheSymbolScreeningDefaultsWhenNothingIsSet(t *testing.T) {
	assert.Equal(t, config.SymbolScreeningConfig{
		MaximumCandidatesPerRun: 20,
		TimeAllowance:           120 * time.Minute,
		PollInterval:            5 * time.Second,
		RunRetention:            90 * 24 * time.Hour,
		MarketDataRetention:     30 * 24 * time.Hour,
	}, config.Load().SymbolScreening)
}

func TestLoadReadsTheSymbolScreeningSettings(t *testing.T) {
	t.Setenv("SYMBOL_SCREENING_MAX_CANDIDATES_PER_RUN", "10")
	t.Setenv("SYMBOL_SCREENING_RUN_TIME_ALLOWANCE_MINUTES", "60")
	t.Setenv("SYMBOL_SCREENING_RUN_POLL_INTERVAL_SECONDS", "30")
	t.Setenv("SYMBOL_SCREENING_RUN_RETENTION_DAYS", "14")
	t.Setenv("SYMBOL_SCREENING_MARKET_DATA_RETENTION_DAYS", "7")

	assert.Equal(t, config.SymbolScreeningConfig{
		MaximumCandidatesPerRun: 10,
		TimeAllowance:           60 * time.Minute,
		PollInterval:            30 * time.Second,
		RunRetention:            14 * 24 * time.Hour,
		MarketDataRetention:     7 * 24 * time.Hour,
	}, config.Load().SymbolScreening)
}

func TestLoadFallsBackToTheScreeningDefaultsForUnusableSettings(t *testing.T) {
	t.Setenv("SYMBOL_SCREENING_MAX_CANDIDATES_PER_RUN", "0")
	t.Setenv("SYMBOL_SCREENING_RUN_POLL_INTERVAL_SECONDS", "-1")

	screening := config.Load().SymbolScreening

	assert.Equal(t, 20, screening.MaximumCandidatesPerRun)
	assert.Equal(t, 5*time.Second, screening.PollInterval)
}

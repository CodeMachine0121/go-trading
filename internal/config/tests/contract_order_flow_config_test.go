package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestLoadAppliesTheContractOrderFlowDefaultsWhenNothingIsSet(t *testing.T) {
	applicationConfig := config.Load()

	assert.Equal(t, config.ContractOrderFlowConfig{
		ArchiveBaseUrl:             "https://data.binance.vision/data/futures/um/daily/aggTrades",
		LiveTradesUrl:              "https://fapi.binance.com/fapi/v1/aggTrades",
		ArchiveTimeout:             180 * time.Second,
		LiveRequestsPerMinute:      40,
		MemoryBytes:                256 << 20,
		ConcurrentArchiveDownloads: 2,
		BotPreparationAllowance:    40 * time.Second,
	}, applicationConfig.ContractOrderFlow)
}

func TestLoadReadsTheContractOrderFlowSettings(t *testing.T) {
	t.Setenv("CONTRACT_ORDER_FLOW_ARCHIVE_BASE_URL", "https://archive.example.com")
	t.Setenv("CONTRACT_ORDER_FLOW_TRADES_URL", "https://trades.example.com")
	t.Setenv("CONTRACT_ORDER_FLOW_ARCHIVE_TIMEOUT_SECONDS", "60")
	t.Setenv("CONTRACT_ORDER_FLOW_REQUESTS_PER_MINUTE", "20")
	t.Setenv("CONTRACT_ORDER_FLOW_MEMORY_MEGABYTES", "512")
	t.Setenv("CONTRACT_ORDER_FLOW_CONCURRENT_ARCHIVE_DOWNLOADS", "1")
	t.Setenv("CONTRACT_ORDER_FLOW_BOT_PREPARATION_SECONDS", "20")

	applicationConfig := config.Load()

	assert.Equal(t, config.ContractOrderFlowConfig{
		ArchiveBaseUrl:             "https://archive.example.com",
		LiveTradesUrl:              "https://trades.example.com",
		ArchiveTimeout:             60 * time.Second,
		LiveRequestsPerMinute:      20,
		MemoryBytes:                512 << 20,
		ConcurrentArchiveDownloads: 1,
		BotPreparationAllowance:    20 * time.Second,
	}, applicationConfig.ContractOrderFlow)
}

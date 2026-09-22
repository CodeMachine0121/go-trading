package config_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestContractIngestionDefaultsPointAtTheContractVenue(t *testing.T) {
	applicationConfig := config.Load()

	contractIngestion := applicationConfig.ContractIngestion
	assert.Contains(t, contractIngestion.BaseUrl, "fapi")
	assert.Contains(t, contractIngestion.MarkPriceUrl, "markPriceKlines")
	assert.Contains(t, contractIngestion.SymbolCatalogUrl, "fapi")
	assert.Equal(t, 10*time.Second, contractIngestion.RequestTimeout)
	assert.Equal(t, 25, contractIngestion.RoundCandleCount)
	assert.Equal(t, 24*time.Hour, contractIngestion.BackfillLookback)
	assert.Equal(t, 3650, contractIngestion.HistorySyncMaxLookbackDays)
}

func TestTheContractAllowanceDefaultsToHalfTheSpotOne(t *testing.T) {
	// Every contract candle costs two requests rather than one, so the same number of
	// candles a minute is reached from half the number written down.
	applicationConfig := config.Load()

	assert.Equal(t, 600, applicationConfig.Ingestion.MarketDataRequestsPerMinute)
	assert.Equal(t, 300, applicationConfig.ContractIngestion.RequestsPerMinute)
}

func TestTheTwoVenuesAreSettledApart(t *testing.T) {
	// Each of these is the contract venue's own number. Sharing the spot one would
	// point this side at the wrong address, spend half the allowance it is allowed,
	// or offer a history the venue does not have.
	t.Setenv("CONTRACT_MARKET_DATA_BASE_URL", "https://example.test/contract/klines")
	t.Setenv("CONTRACT_MARKET_DATA_MARK_PRICE_URL", "https://example.test/contract/mark")
	t.Setenv("CONTRACT_MARKET_DATA_SYMBOL_CATALOG_URL", "https://example.test/contract/catalog")
	t.Setenv("CONTRACT_MARKET_DATA_REQUESTS_PER_MINUTE", "111")
	t.Setenv("CONTRACT_MARKET_DATA_REQUEST_TIMEOUT_SECONDS", "7")
	t.Setenv("CONTRACT_KCANDLE_INGESTION_ROUND_CANDLE_COUNT", "9")
	t.Setenv("CONTRACT_KCANDLE_INGESTION_BACKFILL_LOOKBACK_HOURS", "5")
	t.Setenv("CONTRACT_KCANDLE_HISTORY_SYNC_MAX_LOOKBACK_DAYS", "2000")

	applicationConfig := config.Load()

	contractIngestion := applicationConfig.ContractIngestion
	assert.Equal(t, "https://example.test/contract/klines", contractIngestion.BaseUrl)
	assert.Equal(t, "https://example.test/contract/mark", contractIngestion.MarkPriceUrl)
	assert.Equal(t, "https://example.test/contract/catalog", contractIngestion.SymbolCatalogUrl)
	assert.Equal(t, 111, contractIngestion.RequestsPerMinute)
	assert.Equal(t, 7*time.Second, contractIngestion.RequestTimeout)
	assert.Equal(t, 9, contractIngestion.RoundCandleCount)
	assert.Equal(t, 5*time.Hour, contractIngestion.BackfillLookback)
	assert.Equal(t, 2000, contractIngestion.HistorySyncMaxLookbackDays)

	// None of it reached the spot side.
	spotIngestion := applicationConfig.Ingestion
	assert.Contains(t, spotIngestion.MarketDataBaseUrl, "api.binance.com")
	assert.Equal(t, 600, spotIngestion.MarketDataRequestsPerMinute)
	assert.Equal(t, 25, spotIngestion.RoundCandleCount)
	assert.Equal(t, 3650, spotIngestion.HistorySyncMaxLookbackDays)
}

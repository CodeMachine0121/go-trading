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

func TestTheContractSupplementsDefaultToTheVenueAndTheirOwnPace(t *testing.T) {
	contractIngestion := config.Load().ContractIngestion

	assert.Equal(t, "https://fapi.binance.com/fapi/v1/indexPriceKlines", contractIngestion.IndexPriceUrl)
	assert.Equal(t, "https://fapi.binance.com/fapi/v1/premiumIndexKlines", contractIngestion.PremiumIndexUrl)
	assert.Equal(t, "https://fapi.binance.com/fapi/v1/fundingRate", contractIngestion.FundingRateUrl)
	assert.Equal(t, "https://fapi.binance.com/fapi/v1/fundingInfo", contractIngestion.FundingInfoUrl)
	assert.Equal(t, "https://fapi.binance.com/futures/data", contractIngestion.StatisticsBaseUrl)
	assert.Equal(t, 180, contractIngestion.StatisticsRequestsPerMinute)
	assert.Equal(t, "https://data.binance.vision/data/futures/um/daily/metrics",
		contractIngestion.PositionStatisticArchiveBaseUrl)
	assert.Equal(t, 120, contractIngestion.PositionStatisticArchiveRequestsPerMinute)
	assert.Equal(t, time.Hour, contractIngestion.FundingRateIngestionInterval)
	assert.Equal(t, 5*time.Minute, contractIngestion.PositionStatisticIngestionInterval)
	assert.Equal(t, 24*time.Hour, contractIngestion.TradingSpecificationRefreshInterval)
}

func TestTheContractSupplementsAreSettable(t *testing.T) {
	t.Setenv("CONTRACT_MARKET_DATA_INDEX_PRICE_URL", "https://example.test/index")
	t.Setenv("CONTRACT_MARKET_DATA_PREMIUM_INDEX_URL", "https://example.test/premium")
	t.Setenv("CONTRACT_MARKET_DATA_FUNDING_RATE_URL", "https://example.test/funding")
	t.Setenv("CONTRACT_MARKET_DATA_FUNDING_INFO_URL", "https://example.test/funding-info")
	t.Setenv("CONTRACT_MARKET_DATA_STATISTICS_BASE_URL", "https://example.test/statistics")
	t.Setenv("CONTRACT_MARKET_DATA_STATISTICS_REQUESTS_PER_MINUTE", "90")
	t.Setenv("CONTRACT_MARKET_DATA_POSITION_STATISTIC_ARCHIVE_BASE_URL", "https://example.test/archive")
	t.Setenv("CONTRACT_MARKET_DATA_POSITION_STATISTIC_ARCHIVE_REQUESTS_PER_MINUTE", "40")
	t.Setenv("CONTRACT_FUNDING_RATE_INGESTION_INTERVAL_MINUTES", "30")
	t.Setenv("CONTRACT_POSITION_STATISTIC_INGESTION_INTERVAL_MINUTES", "10")
	t.Setenv("CONTRACT_TRADING_SPECIFICATION_REFRESH_INTERVAL_HOURS", "6")

	contractIngestion := config.Load().ContractIngestion

	assert.Equal(t, "https://example.test/index", contractIngestion.IndexPriceUrl)
	assert.Equal(t, "https://example.test/premium", contractIngestion.PremiumIndexUrl)
	assert.Equal(t, "https://example.test/funding", contractIngestion.FundingRateUrl)
	assert.Equal(t, "https://example.test/funding-info", contractIngestion.FundingInfoUrl)
	assert.Equal(t, "https://example.test/statistics", contractIngestion.StatisticsBaseUrl)
	assert.Equal(t, 90, contractIngestion.StatisticsRequestsPerMinute)
	assert.Equal(t, "https://example.test/archive", contractIngestion.PositionStatisticArchiveBaseUrl)
	assert.Equal(t, 40, contractIngestion.PositionStatisticArchiveRequestsPerMinute)
	assert.Equal(t, 30*time.Minute, contractIngestion.FundingRateIngestionInterval)
	assert.Equal(t, 10*time.Minute, contractIngestion.PositionStatisticIngestionInterval)
	assert.Equal(t, 6*time.Hour, contractIngestion.TradingSpecificationRefreshInterval)
}

func TestAContractSupplementRoundIsSwitchedOffByZeroOrLess(t *testing.T) {
	testCases := []struct {
		name     string
		value    string
		expected time.Duration
	}{
		{name: "零停用", value: "0", expected: 0},
		{name: "負值停用", value: "-5", expected: 0},
		{name: "讀不懂就用預設", value: "often", expected: time.Hour},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("CONTRACT_FUNDING_RATE_INGESTION_INTERVAL_MINUTES", testCase.value)

			assert.Equal(t, testCase.expected, config.Load().ContractIngestion.FundingRateIngestionInterval)
		})
	}
}

func TestTheContractAccountHasNoKeyUnlessOneIsGiven(t *testing.T) {
	t.Setenv("CONTRACT_ACCOUNT_API_KEY", "")
	t.Setenv("CONTRACT_ACCOUNT_API_SECRET", "")

	contractIngestion := config.Load().ContractIngestion

	assert.Empty(t, contractIngestion.AccountApiKey)
	assert.Empty(t, contractIngestion.AccountApiSecret)
	assert.False(t, contractIngestion.HasAccountCredentials())
	assert.Equal(t, "https://fapi.binance.com/fapi/v1/leverageBracket", contractIngestion.MaintenanceMarginTierUrl)
	assert.Equal(t, 24*time.Hour, contractIngestion.MaintenanceMarginTierRefreshInterval)
}

func TestTheContractAccountNeedsBothHalvesOfItsKey(t *testing.T) {
	testCases := []struct {
		name     string
		key      string
		secret   string
		expected bool
	}{
		{name: "兩半都有", key: "key", secret: "secret", expected: true},
		{name: "只有金鑰", key: "key", expected: false},
		{name: "只有密鑰", secret: "secret", expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("CONTRACT_ACCOUNT_API_KEY", testCase.key)
			t.Setenv("CONTRACT_ACCOUNT_API_SECRET", testCase.secret)
			t.Setenv("CONTRACT_MARKET_DATA_MAINTENANCE_MARGIN_TIER_URL", "https://example.test/brackets")
			t.Setenv("CONTRACT_MAINTENANCE_MARGIN_TIER_REFRESH_INTERVAL_HOURS", "12")

			contractIngestion := config.Load().ContractIngestion

			assert.Equal(t, testCase.expected, contractIngestion.HasAccountCredentials())
			assert.Equal(t, testCase.key, contractIngestion.AccountApiKey)
			assert.Equal(t, "https://example.test/brackets", contractIngestion.MaintenanceMarginTierUrl)
			assert.Equal(t, 12*time.Hour, contractIngestion.MaintenanceMarginTierRefreshInterval)
		})
	}
}

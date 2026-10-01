package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBinanceTradingKeyDomainTrimsBothStrings(t *testing.T) {
	tradingKey, err := domains.NewBinanceTradingKeyDomain(dto.BinanceTradingKeyWriteDto{
		ApiKey: "  the-api-key-a1b2\n", SecretKey: "\tthe-secret-key ",
	})

	require.NoError(t, err)
	credential := tradingKey.ToCredentialVo()
	assert.Equal(t, "the-api-key-a1b2", credential.ApiKey)
	assert.Equal(t, "the-secret-key", credential.SecretKey)
	assert.Equal(t, "a1b2", tradingKey.ToEntity(
		1, "sealed-api", "sealed-secret", domains.NewTradableMarketsDomain(true, false)).ApiKeyTail)
}

func TestNewBinanceTradingKeyDomainRefusesBlankStrings(t *testing.T) {
	testCases := []struct {
		name            string
		apiKey          string
		secretKey       string
		expectedMessage string
	}{
		{name: "a blank API key", apiKey: "   ", secretKey: "the-secret-key", expectedMessage: "必須給 API Key"},
		{name: "a blank secret key", apiKey: "the-api-key", secretKey: " \t", expectedMessage: "必須給 Secret Key"},
		{name: "both blank names the API key first", apiKey: "", secretKey: "", expectedMessage: "必須給 API Key"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewBinanceTradingKeyDomain(dto.BinanceTradingKeyWriteDto{
				ApiKey: testCase.apiKey, SecretKey: testCase.secretKey,
			})

			require.ErrorIs(t, err, domains.ErrBinanceTradingKeyValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestBinanceTradingKeyDomainToEntityKeepsNoPlainStringAndRecordsMarkets(t *testing.T) {
	testCases := []struct {
		name             string
		apiKey           string
		tradableMarkets  domains.TradableMarketsDomain
		expectedTail     string
		expectedSpot     bool
		expectedContract bool
	}{
		{
			name: "spot only", apiKey: "the-api-key-a1b2",
			tradableMarkets: domains.NewTradableMarketsDomain(true, false),
			expectedTail:    "a1b2", expectedSpot: true, expectedContract: false,
		},
		{
			name: "both markets", apiKey: "the-api-key-c3d4",
			tradableMarkets: domains.NewTradableMarketsDomain(true, true),
			expectedTail:    "c3d4", expectedSpot: true, expectedContract: true,
		},
		{
			// A key no longer than the tail would be shown whole, so nothing is shown.
			name: "a four-character key shows no tail", apiKey: "a1b2",
			tradableMarkets: domains.NewTradableMarketsDomain(false, true),
			expectedTail:    "", expectedSpot: false, expectedContract: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingKey, err := domains.NewBinanceTradingKeyDomain(dto.BinanceTradingKeyWriteDto{
				ApiKey: testCase.apiKey, SecretKey: "the-secret-key",
			})
			require.NoError(t, err)

			binanceTradingKey := tradingKey.ToEntity(7, "sealed-api", "sealed-secret", testCase.tradableMarkets)

			assert.Equal(t, uint(7), binanceTradingKey.UserID)
			assert.Equal(t, "sealed-api", binanceTradingKey.SealedApiKey)
			assert.Equal(t, "sealed-secret", binanceTradingKey.SealedSecretKey)
			assert.Equal(t, testCase.expectedTail, binanceTradingKey.ApiKeyTail)
			assert.Equal(t, testCase.expectedSpot, binanceTradingKey.SpotTradingEnabled)
			assert.Equal(t, testCase.expectedContract, binanceTradingKey.ContractTradingEnabled)
		})
	}
}

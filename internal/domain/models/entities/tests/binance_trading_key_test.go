package entities_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aStoredBinanceTradingKey(spotTradingEnabled bool, contractTradingEnabled bool) entities.BinanceTradingKey {
	return entities.BinanceTradingKey{
		ID: 1, UserID: 7,
		SealedApiKey: "sealed-api-key", SealedSecretKey: "sealed-secret-key",
		ApiKeyTail:             "a1b2",
		SpotTradingEnabled:     spotTradingEnabled,
		ContractTradingEnabled: contractTradingEnabled,
		UpdatedAt:              time.Date(2026, 10, 1, 8, 0, 0, 0, time.FixedZone("Asia/Taipei", 8*3600)),
	}
}

func TestBinanceTradingKeyToDtoShowsTheTailMarketsAndTimeOnly(t *testing.T) {
	binanceTradingKeyDto := aStoredBinanceTradingKey(true, true).ToDto()

	assert.True(t, binanceTradingKeyDto.Configured)
	assert.Equal(t, "a1b2", binanceTradingKeyDto.ApiKeyTail)
	assert.Equal(t, []string{"spot", "contract"}, binanceTradingKeyDto.TradableMarkets)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), binanceTradingKeyDto.ConfiguredAt)

	serialized, marshalError := json.Marshal(binanceTradingKeyDto)
	require.NoError(t, marshalError)
	assert.NotContains(t, string(serialized), "sealed-secret-key")
	assert.NotContains(t, string(serialized), "sealed-api-key")
}

func TestBinanceTradingKeyToStatusDtoShowsNoPartOfTheKey(t *testing.T) {
	statusDto := aStoredBinanceTradingKey(false, true).ToStatusDto()

	assert.True(t, statusDto.Configured)
	assert.Equal(t, []string{"contract"}, statusDto.TradableMarkets)
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), statusDto.ConfiguredAt)

	serialized, marshalError := json.Marshal(statusDto)
	require.NoError(t, marshalError)
	assert.NotContains(t, string(serialized), "a1b2")
	assert.NotContains(t, string(serialized), "sealed")
}

func TestBinanceTradingKeyListsSpotOnlyWhenOnlySpotIsAllowed(t *testing.T) {
	assert.Equal(t, []string{"spot"}, aStoredBinanceTradingKey(true, false).ToDto().TradableMarkets)
}

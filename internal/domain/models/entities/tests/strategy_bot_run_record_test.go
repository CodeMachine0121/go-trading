package entities_test

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStrategyBotRunRecordToDtoCarriesWhatAContractRoundSuggested(t *testing.T) {
	runRecord := StrategyBotRunRecord{
		RunNumber: 7, RanAt: time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC), Result: "sell",
		SuggestedStake:     decimal.NewNullDecimal(decimal.NewFromInt(1000)),
		SuggestedDirection: "short",
		SuggestedLeverage:  decimal.NewNullDecimal(decimal.NewFromInt(5)),
		SuggestedNotional:  decimal.NewNullDecimal(decimal.NewFromInt(5000)),
	}

	wire, marshalError := json.Marshal(runRecord.ToDto())

	require.NoError(t, marshalError)
	assert.Contains(t, string(wire), `"suggestedDirection":"short"`)
	assert.Contains(t, string(wire), `"suggestedLeverage":"5"`)
	assert.Contains(t, string(wire), `"suggestedNotional":"5000"`)
}

// A spot round, and every round stored before these were remembered, leaves all three
// off the wire rather than sending them empty.
func TestStrategyBotRunRecordToDtoLeavesASpotRoundAsItWas(t *testing.T) {
	runRecord := StrategyBotRunRecord{
		RunNumber: 7, RanAt: time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC), Result: "buy",
		SuggestedStake: decimal.NewNullDecimal(decimal.NewFromInt(5000)),
	}

	wire, marshalError := json.Marshal(runRecord.ToDto())

	require.NoError(t, marshalError)
	assert.JSONEq(t, `{"runNumber":7,"ranAt":"2026-09-24T08:00:00Z","result":"buy","suggestedStake":"5000"}`,
		string(wire))
}

package main

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheAssistantIsOfferedNothingItMustNotDo pins the assistant's capabilities: deleting scripts or
// strategies and touching bots are deliberately absent rather than guarded, since a lost algorithm is unrecoverable.
func TestTheAssistantIsOfferedNothingItMustNotDo(t *testing.T) {
	assistantQueries := assistantQueriesFor(nil, nil, nil, nil, nil, nil, nil, 200)

	offeredNames := make([]string, 0, len(assistantQueries))
	for _, assistantQuery := range assistantQueries {
		offeredNames = append(offeredNames, assistantQuery.Name())
	}
	slices.Sort(offeredNames)

	assert.Equal(t, []string{
		"calculate_indicator",
		"create_strategy_script",
		"create_trading_strategy",
		"get_k_candle_series",
		"get_k_candles",
		"get_strategy_script",
		"get_trading_strategy",
		"list_strategy_scripts",
		"list_trading_strategies",
		"list_trading_symbols",
		"run_trading_strategy_backtest",
		"update_strategy_script",
		"update_trading_strategy",
	}, offeredNames)
}

func TestEveryOfferedCapabilityIsUsable(t *testing.T) {
	assistantQueries := assistantQueriesFor(nil, nil, nil, nil, nil, nil, nil, 200)

	for _, assistantQuery := range assistantQueries {
		t.Run(assistantQuery.Name(), func(t *testing.T) {
			assert.NotEmpty(t, assistantQuery.Name())
			assert.NotEmpty(t, assistantQuery.Description())

			schema := struct {
				Type       string                     `json:"type"`
				Properties map[string]json.RawMessage `json:"properties"`
			}{}
			require.NoError(t, json.Unmarshal([]byte(assistantQuery.ArgumentSchema()), &schema),
				"參數格式必須是合法 JSON，否則助手拿到的是一個叫不動的能力")
			assert.Equal(t, "object", schema.Type)
		})
	}
}

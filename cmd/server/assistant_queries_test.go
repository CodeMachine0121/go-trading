package main

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheAssistantIsOfferedNothingItMustNotDo holds the boundary that the assistant's
// reach is the list it is handed and nothing else.
//
// Not being able to delete a strategy script is the whole point: a strategy script saved by mistake
// costs a name, one deleted by mistake costs an algorithm that took several sittings
// to get right and cannot be recovered. Written as a guard clause somewhere in the
// loop it would be one refactor away from gone; written as a missing capability there
// is nothing to reach for.
//
// A trading strategy is the sharper case of the same rule: it is several scripts and
// two condition trees that took a few rounds to converge, so deleting one costs far
// more than deleting a script. Nothing here touches a bot either — starting somebody's
// machine is not the assistant's to do.
func TestTheAssistantIsOfferedNothingItMustNotDo(t *testing.T) {
	assistantQueries := assistantQueriesFor(nil, nil, nil, nil, nil, nil, 200)

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

// TestEveryOfferedCapabilityIsUsable holds that a capability the assistant is told
// about can actually be called: a name it can ask for, a description telling it when
// to, and arguments it can read. An assistant offered a capability it cannot call
// correctly will keep calling it incorrectly, and pay for every attempt.
func TestEveryOfferedCapabilityIsUsable(t *testing.T) {
	assistantQueries := assistantQueriesFor(nil, nil, nil, nil, nil, nil, 200)

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

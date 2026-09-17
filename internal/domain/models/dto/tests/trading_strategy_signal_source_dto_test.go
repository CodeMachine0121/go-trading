package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A bot's knob values go out and come straight back in: the screen that reads a bot
// is the screen that saves it, so every name it cannot read is a name it cannot hand
// back — and a write naming a knob the strategy script never declared is refused. The
// refusal names the knob, three steps away from the field that lost it, so the shape
// this goes out in is worth pinning rather than reading off the struct.
func TestTradingStrategySignalSourceDtoIsWrittenInTheShapeItIsReadIn(t *testing.T) {
	signalSource := dto.TradingStrategySignalSourceDto{
		Label:               "A",
		StrategyScriptID:    30,
		AggregationInterval: "5m",
		ParameterValues: []dto.StrategyScriptParameterValueDto{
			{Name: "快線期數", Value: 10},
		},
	}

	written, marshalError := json.Marshal(signalSource)
	require.NoError(t, marshalError)

	assert.JSONEq(t, `{
		"label": "A",
		"strategyScriptId": 30,
		"aggregationInterval": "5m",
		"parameterValues": [{"name": "快線期數", "value": 10}]
	}`, string(written))
}

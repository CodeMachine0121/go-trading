package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Parameter values must serialize with the names they are read by, since bots round-trip
// them and unknown names are refused.
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

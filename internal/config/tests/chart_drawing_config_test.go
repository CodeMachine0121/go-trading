package config_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestLoadReadsTheChartDrawingLimits(t *testing.T) {
	cases := []struct {
		name          string
		boardCapacity string
		batchCapacity string
		expected      vo.ChartDrawingLimitsVo
	}{
		{name: "nothing set", expected: vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20}},
		{name: "both set", boardCapacity: "50", batchCapacity: "5",
			expected: vo.ChartDrawingLimitsVo{BoardCapacity: 50, AssistantBatchCapacity: 5}},
		{name: "unusable values", boardCapacity: "0", batchCapacity: "-3",
			expected: vo.ChartDrawingLimitsVo{BoardCapacity: 200, AssistantBatchCapacity: 20}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("CHART_DRAWING_BOARD_CAPACITY", testCase.boardCapacity)
			t.Setenv("CHART_DRAWING_ASSISTANT_BATCH_CAPACITY", testCase.batchCapacity)

			assert.Equal(t, testCase.expected, config.Load().ChartDrawingLimits)
		})
	}
}

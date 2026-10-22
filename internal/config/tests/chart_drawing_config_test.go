package config_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/config"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func TestLoadReadsTheChartDrawingLimits(t *testing.T) {
	cases := []struct {
		name                string
		boardCapacity       string
		boardAnchorCapacity string
		batchCapacity       string
		expected            vo.ChartDrawingLimitsVo
	}{
		{name: "nothing set",
			expected: vo.ChartDrawingLimitsVo{BoardCapacity: 200, BoardAnchorCapacity: 4000, AssistantBatchCapacity: 20}},
		{name: "all set", boardCapacity: "50", boardAnchorCapacity: "1000", batchCapacity: "5",
			expected: vo.ChartDrawingLimitsVo{BoardCapacity: 50, BoardAnchorCapacity: 1000, AssistantBatchCapacity: 5}},
		{name: "unusable values", boardCapacity: "0", boardAnchorCapacity: "-1", batchCapacity: "-3",
			expected: vo.ChartDrawingLimitsVo{BoardCapacity: 200, BoardAnchorCapacity: 4000, AssistantBatchCapacity: 20}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("CHART_DRAWING_BOARD_CAPACITY", testCase.boardCapacity)
			t.Setenv("CHART_DRAWING_BOARD_ANCHOR_CAPACITY", testCase.boardAnchorCapacity)
			t.Setenv("CHART_DRAWING_ASSISTANT_BATCH_CAPACITY", testCase.batchCapacity)

			assert.Equal(t, testCase.expected, config.Load().ChartDrawingLimits)
		})
	}
}

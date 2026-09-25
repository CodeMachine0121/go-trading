package entities_test

import (
	"testing"
	"time"

	. "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishedStrategyScriptShowsEverythingButTheAlgorithm(t *testing.T) {
	publishedAt := time.Date(2026, 9, 10, 8, 0, 0, 0, time.FixedZone("Asia/Taipei", 8*60*60))
	publication := PublishedStrategyScript{
		StrategyScriptID: 7,
		PublishedAt:      publishedAt,
		StrategyScript: StrategyScript{
			ID:          7,
			Name:        "二十根均線",
			Description: "抓短線轉折",
			Script:      "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
			ResultType:  "floatList",
			Owner:       User{ID: 1, Email: "owner@example.com"},
			Parameters: []StrategyScriptParameter{
				{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
			},
		},
	}

	publishedStrategyScriptDto := publication.ToDto()

	assert.Equal(t, uint(7), publishedStrategyScriptDto.ID)
	assert.Equal(t, "二十根均線", publishedStrategyScriptDto.Name)
	assert.Equal(t, "抓短線轉折", publishedStrategyScriptDto.Description)
	assert.Equal(t, "floatList", publishedStrategyScriptDto.ResultType)
	assert.Equal(t, "owner@example.com", publishedStrategyScriptDto.PublisherEmail)
	assert.Equal(t, publishedAt.UTC(), publishedStrategyScriptDto.PublishedAt,
		"a moment is handed out in universal time whatever zone it was read back in")
	require.Len(t, publishedStrategyScriptDto.Parameters, 1,
		"a knob is a name and a default, not a step — declaring it gives nothing away")
	assert.Equal(t, "lookback", publishedStrategyScriptDto.Parameters[0].Name)
}

func TestPublishedStrategyScriptWithNoKnobsHandsOutAnEmptyList(t *testing.T) {
	publication := PublishedStrategyScript{StrategyScriptID: 7, StrategyScript: StrategyScript{ID: 7, Name: "甲"}}

	publishedStrategyScriptDto := publication.ToDto()

	assert.NotNil(t, publishedStrategyScriptDto.Parameters)
	assert.Empty(t, publishedStrategyScriptDto.Parameters)
}

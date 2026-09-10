package entities_test

import (
	"testing"
	"time"

	. "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublishedStrategyShowsEverythingButTheAlgorithm(t *testing.T) {
	publishedAt := time.Date(2026, 9, 10, 8, 0, 0, 0, time.FixedZone("Asia/Taipei", 8*60*60))
	publication := PublishedStrategy{
		StrategyID:  7,
		PublishedAt: publishedAt,
		Strategy: Strategy{
			ID:          7,
			Name:        "二十根均線",
			Description: "抓短線轉折",
			Script:      "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
			ResultType:  "floatList",
			Owner:       User{ID: 1, Email: "owner@example.com"},
			Parameters: []StrategyParameter{
				{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
			},
		},
	}

	publishedStrategyDto := publication.ToDto()

	assert.Equal(t, uint(7), publishedStrategyDto.ID)
	assert.Equal(t, "二十根均線", publishedStrategyDto.Name)
	assert.Equal(t, "抓短線轉折", publishedStrategyDto.Description)
	assert.Equal(t, "floatList", publishedStrategyDto.ResultType)
	assert.Equal(t, "owner@example.com", publishedStrategyDto.PublisherEmail)
	assert.Equal(t, publishedAt.UTC(), publishedStrategyDto.PublishedAt,
		"a moment is handed out in universal time whatever zone it was read back in")
	require.Len(t, publishedStrategyDto.Parameters, 1,
		"a knob is a name and a default, not a step — declaring it gives nothing away")
	assert.Equal(t, "lookback", publishedStrategyDto.Parameters[0].Name)
}

func TestPublishedStrategyWithNoKnobsHandsOutAnEmptyList(t *testing.T) {
	// An absence and an empty list read the same to a person and differently to a
	// caller, and only one of them can be looped over without checking first.
	publication := PublishedStrategy{StrategyID: 7, Strategy: Strategy{ID: 7, Name: "甲"}}

	publishedStrategyDto := publication.ToDto()

	assert.NotNil(t, publishedStrategyDto.Parameters)
	assert.Empty(t, publishedStrategyDto.Parameters)
}

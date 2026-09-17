package entities_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
)

func TestStrategyScriptToDto(t *testing.T) {
	createdAt := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 9, 3, 9, 30, 0, 0, time.UTC)

	strategyScript := entities.StrategyScript{
		ID:         7,
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType: "floatList",
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}

	strategyScriptDto := strategyScript.ToDto()

	assert.Equal(t, uint(7), strategyScriptDto.ID)
	assert.Equal(t, "二十根均線", strategyScriptDto.Name)
	assert.Equal(t, strategyScript.Script, strategyScriptDto.Script)
	assert.Equal(t, "floatList", strategyScriptDto.ResultType)
	assert.Equal(t, createdAt, strategyScriptDto.CreatedAt)
	assert.Equal(t, updatedAt, strategyScriptDto.UpdatedAt)
}

func TestStrategyScriptToDtoHandsOutBothTimesInUniversalTime(t *testing.T) {
	eightHoursAhead := time.FixedZone("UTC+8", 8*60*60)

	strategyScript := entities.StrategyScript{
		CreatedAt: time.Date(2026, 9, 3, 16, 0, 0, 0, eightHoursAhead),
		UpdatedAt: time.Date(2026, 9, 3, 17, 30, 0, 0, eightHoursAhead),
	}

	strategyScriptDto := strategyScript.ToDto()

	assert.Equal(t, time.UTC, strategyScriptDto.CreatedAt.Location())
	assert.Equal(t, time.UTC, strategyScriptDto.UpdatedAt.Location())
	assert.Equal(t, time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC), strategyScriptDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 3, 9, 30, 0, 0, time.UTC), strategyScriptDto.UpdatedAt)
}

func TestStrategyScriptTableName(t *testing.T) {
	assert.Equal(t, "Strategies", entities.StrategyScript{}.TableName())
}

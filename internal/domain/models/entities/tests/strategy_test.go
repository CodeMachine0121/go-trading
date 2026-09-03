package entities_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
)

func TestStrategyToDto(t *testing.T) {
	createdAt := time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 9, 3, 9, 30, 0, 0, time.UTC)

	strategy := entities.Strategy{
		ID:         7,
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType: "floatList",
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}

	strategyDto := strategy.ToDto()

	assert.Equal(t, uint(7), strategyDto.ID)
	assert.Equal(t, "二十根均線", strategyDto.Name)
	assert.Equal(t, strategy.Script, strategyDto.Script)
	assert.Equal(t, "floatList", strategyDto.ResultType)
	assert.Equal(t, createdAt, strategyDto.CreatedAt)
	assert.Equal(t, updatedAt, strategyDto.UpdatedAt)
}

func TestStrategyToDtoHandsOutBothTimesInUniversalTime(t *testing.T) {
	eightHoursAhead := time.FixedZone("UTC+8", 8*60*60)

	strategy := entities.Strategy{
		CreatedAt: time.Date(2026, 9, 3, 16, 0, 0, 0, eightHoursAhead),
		UpdatedAt: time.Date(2026, 9, 3, 17, 30, 0, 0, eightHoursAhead),
	}

	strategyDto := strategy.ToDto()

	assert.Equal(t, time.UTC, strategyDto.CreatedAt.Location())
	assert.Equal(t, time.UTC, strategyDto.UpdatedAt.Location())
	assert.Equal(t, time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC), strategyDto.CreatedAt)
	assert.Equal(t, time.Date(2026, 9, 3, 9, 30, 0, 0, time.UTC), strategyDto.UpdatedAt)
}

func TestStrategyTableName(t *testing.T) {
	assert.Equal(t, "Strategies", entities.Strategy{}.TableName())
}

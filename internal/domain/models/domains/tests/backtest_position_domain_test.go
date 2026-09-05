package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var positionEntryTime = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
var positionExitTime = time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC)

func TestNewBacktestPositionDomain(t *testing.T) {
	testCases := []struct {
		name          string
		entryPrice    decimal.Decimal
		stake         decimal.Decimal
		expectsOpened bool
	}{
		{
			name:          "a positive price and stake open a position",
			entryPrice:    decimal.NewFromInt(100),
			stake:         decimal.NewFromInt(10000),
			expectsOpened: true,
		},
		{
			name:          "a price of zero opens nothing",
			entryPrice:    decimal.Zero,
			stake:         decimal.NewFromInt(10000),
			expectsOpened: false,
		},
		{
			name:          "a negative price opens nothing",
			entryPrice:    decimal.NewFromInt(-100),
			stake:         decimal.NewFromInt(10000),
			expectsOpened: false,
		},
		{
			name:          "a stake of zero opens nothing",
			entryPrice:    decimal.NewFromInt(100),
			stake:         decimal.Zero,
			expectsOpened: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, isOpened := domains.NewBacktestPositionDomain(
				vo.PositionDirectionLong, positionEntryTime, testCase.entryPrice, testCase.stake)

			assert.Equal(t, testCase.expectsOpened, isOpened)
		})
	}
}

func TestBacktestPositionDomainValuation(t *testing.T) {
	testCases := []struct {
		name           string
		direction      vo.PositionDirectionVo
		entryPrice     decimal.Decimal
		stake          decimal.Decimal
		price          decimal.Decimal
		expectedProfit decimal.Decimal
		expectedValue  decimal.Decimal
	}{
		{
			name:           "a long gains what the price gained",
			direction:      vo.PositionDirectionLong,
			entryPrice:     decimal.NewFromInt(100),
			stake:          decimal.NewFromInt(10000),
			price:          decimal.NewFromInt(110),
			expectedProfit: decimal.NewFromInt(1000),
			expectedValue:  decimal.NewFromInt(11000),
		},
		{
			name:           "a long loses what the price lost",
			direction:      vo.PositionDirectionLong,
			entryPrice:     decimal.NewFromInt(100),
			stake:          decimal.NewFromInt(10000),
			price:          decimal.NewFromInt(90),
			expectedProfit: decimal.NewFromInt(-1000),
			expectedValue:  decimal.NewFromInt(9000),
		},
		{
			name:           "a short gains what the price lost",
			direction:      vo.PositionDirectionShort,
			entryPrice:     decimal.NewFromInt(100),
			stake:          decimal.NewFromInt(10000),
			price:          decimal.NewFromInt(90),
			expectedProfit: decimal.NewFromInt(1000),
			expectedValue:  decimal.NewFromInt(11000),
		},
		{
			name:           "a short loses what the price gained",
			direction:      vo.PositionDirectionShort,
			entryPrice:     decimal.NewFromInt(100),
			stake:          decimal.NewFromInt(10000),
			price:          decimal.NewFromInt(120),
			expectedProfit: decimal.NewFromInt(-2000),
			expectedValue:  decimal.NewFromInt(8000),
		},
		{
			name:           "an unmoved price returns exactly the stake",
			direction:      vo.PositionDirectionLong,
			entryPrice:     decimal.NewFromInt(100),
			stake:          decimal.NewFromInt(10000),
			price:          decimal.NewFromInt(100),
			expectedProfit: decimal.Zero,
			expectedValue:  decimal.NewFromInt(10000),
		},
		{
			name:           "only the stake is at work, not the whole account",
			direction:      vo.PositionDirectionLong,
			entryPrice:     decimal.NewFromInt(100),
			stake:          decimal.NewFromInt(3000),
			price:          decimal.NewFromInt(110),
			expectedProfit: decimal.NewFromInt(300),
			expectedValue:  decimal.NewFromInt(3300),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			position, isOpened := domains.NewBacktestPositionDomain(
				testCase.direction, positionEntryTime, testCase.entryPrice, testCase.stake)
			require.True(t, isOpened)

			assert.True(t, testCase.expectedProfit.Equal(position.ProfitAt(testCase.price)),
				"profit was %s", position.ProfitAt(testCase.price))
			assert.True(t, testCase.expectedValue.Equal(position.ValueAt(testCase.price)),
				"value was %s", position.ValueAt(testCase.price))
		})
	}
}

func TestBacktestPositionDomainClosedAt(t *testing.T) {
	t.Run("a closed position reports both ends and what it made", func(t *testing.T) {
		position, isOpened := domains.NewBacktestPositionDomain(
			vo.PositionDirectionLong, positionEntryTime,
			decimal.NewFromInt(100), decimal.NewFromInt(10000))
		require.True(t, isOpened)

		closedTrade := position.ClosedAt(positionExitTime, decimal.NewFromInt(112))

		assert.Equal(t, vo.PositionDirectionLong, closedTrade.Direction)
		assert.Equal(t, positionEntryTime, closedTrade.EntryTime)
		assert.True(t, decimal.NewFromInt(100).Equal(closedTrade.EntryPrice))
		assert.Equal(t, positionExitTime, closedTrade.ExitTime)
		assert.True(t, decimal.NewFromInt(112).Equal(closedTrade.ExitPrice))
		assert.True(t, decimal.NewFromInt(10000).Equal(closedTrade.Stake))
		assert.True(t, decimal.NewFromInt(1200).Equal(closedTrade.Profit),
			"profit was %s", closedTrade.Profit)
	})

	t.Run("breaking even is not a win", func(t *testing.T) {
		position, isOpened := domains.NewBacktestPositionDomain(
			vo.PositionDirectionShort, positionEntryTime,
			decimal.NewFromInt(100), decimal.NewFromInt(10000))
		require.True(t, isOpened)

		closedTrade := position.ClosedAt(positionExitTime, decimal.NewFromInt(100))

		assert.False(t, closedTrade.IsWin())
	})

	t.Run("a profitable trade is a win", func(t *testing.T) {
		position, isOpened := domains.NewBacktestPositionDomain(
			vo.PositionDirectionShort, positionEntryTime,
			decimal.NewFromInt(100), decimal.NewFromInt(10000))
		require.True(t, isOpened)

		closedTrade := position.ClosedAt(positionExitTime, decimal.NewFromInt(90))

		assert.True(t, closedTrade.IsWin())
	})
}

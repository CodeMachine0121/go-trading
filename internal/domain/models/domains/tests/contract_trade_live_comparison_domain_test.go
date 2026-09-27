package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aClosedLiveTrade(openedDay int, closedDay int, leverage int64) dto.ContractTradeRecordDto {
	closedAt := tradeOpenedAt.AddDate(0, 0, closedDay)

	return dto.ContractTradeRecordDto{
		Symbol: "BTCUSDT", Direction: "long", Leverage: decimal.NewFromInt(leverage),
		OpenedAt: tradeOpenedAt.AddDate(0, 0, openedDay), ClosedAt: &closedAt,
		Outcome: dto.ContractTradeOutcomeDto{NetProfit: decimal.NewFromInt(1)},
	}
}

func TestContractTradeLiveComparisonDomainGroups(t *testing.T) {
	groups := domains.NewContractTradeLiveComparisonDomain([]dto.ContractTradeRecordDto{
		aClosedLiveTrade(5, 6, 10),
		aClosedLiveTrade(1, 9, 3),
		aClosedLiveTrade(2, 3, 20),
	}, decimal.RequireFromString("0.05")).Groups()

	require.Len(t, groups, 1)
	request := groups[0].BacktestRequest
	assert.True(t, request.StartTime.Equal(tradeOpenedAt.AddDate(0, 0, 1)))
	assert.True(t, request.EndTime.Equal(tradeOpenedAt.AddDate(0, 0, 9)))
	assert.Equal(t, "3", request.Leverage.String())
	assert.Equal(t, "10000", request.InitialCapital.String())
	assert.Equal(t, "allIn", request.PositionSizingMode)
	assert.Equal(t, "0.05", request.EntryCostPercentage.String())
	assert.Equal(t, "0.05", request.ExitCostPercentage.String())
}

func TestContractTradeLiveComparisonDomainComposeWithoutAnAttempt(t *testing.T) {
	comparisonDomain := domains.NewContractTradeLiveComparisonDomain(
		[]dto.ContractTradeRecordDto{aClosedLiveTrade(1, 2, 5)}, decimal.Zero)

	rows := comparisonDomain.Compose(comparisonDomain.Groups(), nil)

	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].Backtest)
	assert.Empty(t, rows[0].BacktestUnavailableReason)
	assert.True(t, rows[0].EndTime.After(rows[0].StartTime.Add(time.Hour)))
}

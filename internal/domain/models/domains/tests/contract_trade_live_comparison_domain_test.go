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
	}, decimal.RequireFromString("0.05")).Plan().Groups

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

	rows := comparisonDomain.Compose(comparisonDomain.Plan().Groups, nil)

	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].Backtest)
	assert.Empty(t, rows[0].BacktestUnavailableReason)
	assert.True(t, rows[0].EndTime.After(rows[0].StartTime.Add(time.Hour)))
}

func slippedBy(trade dto.ContractTradeRecordDto, symbol string, slippagePercentage float64) dto.ContractTradeRecordDto {
	trade.Symbol = symbol
	trade.Outcome.EntrySlippagePercentage = &slippagePercentage

	return trade
}

func TestContractTradeLiveComparisonDomainMeasuresLiveSlippage(t *testing.T) {
	ethereum := aClosedLiveTrade(1, 2, 5)
	ethereum.Symbol = "ETHUSDT"

	plan := domains.NewContractTradeLiveComparisonDomain([]dto.ContractTradeRecordDto{
		slippedBy(aClosedLiveTrade(1, 2, 5), "BTCUSDT", 0.08),
		slippedBy(aClosedLiveTrade(2, 3, 5), "BTCUSDT", 0.06),
		aClosedLiveTrade(3, 4, 5),
		slippedBy(aClosedLiveTrade(4, 5, 5), "SOLUSDT", 0.20),
		ethereum,
	}, decimal.Zero).Plan()

	require.Len(t, plan.Groups, 3)
	bitcoin, ether, solana := plan.Groups[0].Live, plan.Groups[1].Live, plan.Groups[2].Live
	assert.Equal(t, 3, bitcoin.ClosedTradeCount)
	assert.Equal(t, 2, bitcoin.EntrySlippageTradeCount)
	assert.InDelta(t, 0.07, *bitcoin.AverageEntrySlippagePercentage, 0.0001)
	assert.Equal(t, 0, ether.EntrySlippageTradeCount)
	assert.Nil(t, ether.AverageEntrySlippagePercentage)
	assert.InDelta(t, 0.20, *solana.AverageEntrySlippagePercentage, 0.0001)
	assert.Equal(t, 3, plan.EntrySlippageTradeCount)
	assert.InDelta(t, (0.08+0.06+0.20)/3, *plan.AverageEntrySlippagePercentage, 0.0001)
}

func TestContractTradeLiveComparisonDomainWithoutAnySlippage(t *testing.T) {
	plan := domains.NewContractTradeLiveComparisonDomain(
		[]dto.ContractTradeRecordDto{aClosedLiveTrade(1, 2, 5)}, decimal.Zero).Plan()

	assert.Nil(t, plan.AverageEntrySlippagePercentage)
	assert.Equal(t, 0, plan.EntrySlippageTradeCount)
}

package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAContractBotShowsWhatItOpenedItself(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	contractBot := storedBot(vo.StrategyBotRunning)
	contractBot.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	contractBot.WatchedSymbols[0].AutoOrderPositionDirection = "long"
	contractBot.WatchedSymbols[0].AutoOrderPositionQuantity = decimal.RequireFromString("0.002")
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(contractBot, nil)

	botDto, getError := underTest.strategyBotApplication.GetStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, getError)
	require.NotNil(t, botDto.WatchedSymbols[0].AutoOrderPosition)
	assert.Equal(t, "long", botDto.WatchedSymbols[0].AutoOrderPosition.Direction)
	assert.True(t, botDto.WatchedSymbols[0].AutoOrderPosition.Quantity.Equal(decimal.RequireFromString("0.002")))
}

func TestASpotBotShowsNoPositionOfItsOwn(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)

	botDto, getError := underTest.strategyBotApplication.GetStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, getError)
	assert.Nil(t, botDto.WatchedSymbols[0].AutoOrderPosition)
}

func TestTheHistoryShowsEachRoundsAutoOrderBesideIt(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	ranAt := time.Date(2026, 10, 4, 6, 30, 0, 0, time.UTC)
	underTest.strategyBotRunRecordRepository.EXPECT().FindLatestByBot(gomock.Any(), strategyBotID, "").
		Return([]entities.StrategyBotRunRecord{
			{RunNumber: 54, RanAt: ranAt, Result: "sell"},
			{RunNumber: 53, RanAt: ranAt, Result: "buy"},
			{RunNumber: 52, RanAt: ranAt, Result: "buy"},
			{RunNumber: 51, RanAt: ranAt, Result: "hold"},
		}, nil)
	*underTest.storedAutoOrders = []entities.ContractAutoOrder{
		{RunNumber: 52, TargetPosition: "long", Status: string(vo.ContractAutoOrderSettled),
			Outcome: string(vo.ContractAutoOrderFilled), OpenDone: true,
			OpenedQuantity:   decimal.NewNullDecimal(decimal.RequireFromString("0.002")),
			OpenAveragePrice: decimal.NewNullDecimal(decimal.NewFromInt(85000)),
			StopLossPrice:    decimal.NewNullDecimal(decimal.NewFromInt(83725)),
			TakeProfitPrice:  decimal.NewNullDecimal(decimal.NewFromInt(87550))},
		{RunNumber: 53, TargetPosition: "long", Status: string(vo.ContractAutoOrderSettled),
			Outcome: string(vo.ContractAutoOrderNotPlaced), Reason: "餘額不足"},
		{RunNumber: 54, TargetPosition: "flat", Status: string(vo.ContractAutoOrderReady)},
	}

	runRecords, listError := underTest.strategyBotApplication.ListRunRecords(
		context.Background(), strategyBotOwnerID, strategyBotID, "")

	require.NoError(t, listError)
	require.Len(t, runRecords, 4)

	pending := runRecords[0].AutoOrder
	require.NotNil(t, pending)
	assert.Equal(t, "pending", pending.Status)

	refused := runRecords[1].AutoOrder
	require.NotNil(t, refused)
	assert.Equal(t, "notPlaced", refused.Status)
	assert.Equal(t, "餘額不足", refused.Reason)

	filled := runRecords[2].AutoOrder
	require.NotNil(t, filled)
	assert.Equal(t, "filled", filled.Status)
	assert.Equal(t, "做多", filled.Action)
	assert.Equal(t, "long", filled.OpenedDirection)
	assert.True(t, filled.OpenedQuantity.Equal(decimal.RequireFromString("0.002")))
	assert.True(t, filled.OpenAveragePrice.Equal(decimal.NewFromInt(85000)))
	assert.True(t, filled.StopLossPrice.Equal(decimal.NewFromInt(83725)))
	assert.True(t, filled.TakeProfitPrice.Equal(decimal.NewFromInt(87550)))
	// Opened at market before entries were limit orders, so it has no entry to describe.
	assert.Nil(t, filled.EntrySlippageTolerancePercentage)
	assert.Nil(t, filled.ReferencePrice)

	assert.Nil(t, runRecords[3].AutoOrder)
}

func TestTheHistoryIsNotShownWithoutItsAutoOrders(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	underTest.strategyBotRunRecordRepository.EXPECT().FindLatestByBot(gomock.Any(), strategyBotID, "").
		Return([]entities.StrategyBotRunRecord{{RunNumber: 52, Result: "buy"}}, nil)
	readFailure := errors.New("the database went away")
	*underTest.autoOrderReadFailure = readFailure

	_, listError := underTest.strategyBotApplication.ListRunRecords(
		context.Background(), strategyBotOwnerID, strategyBotID, "")

	require.ErrorIs(t, listError, readFailure)
}

func TestTheHistoryShowsHowEachEntryOrderWent(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	ranAt := time.Date(2026, 10, 12, 6, 30, 0, 0, time.UTC)
	underTest.strategyBotRunRecordRepository.EXPECT().FindLatestByBot(gomock.Any(), strategyBotID, "").
		Return([]entities.StrategyBotRunRecord{
			{RunNumber: 62, RanAt: ranAt, Result: "sell"},
			{RunNumber: 61, RanAt: ranAt, Result: "buy"},
			{RunNumber: 60, RanAt: ranAt, Result: "sell"},
		}, nil)
	reference := decimal.NewNullDecimal(decimal.NewFromInt(84154))
	*underTest.storedAutoOrders = []entities.ContractAutoOrder{
		{RunNumber: 60, TargetPosition: "short", Status: string(vo.ContractAutoOrderSettled),
			Outcome: string(vo.ContractAutoOrderFilled), OpenDone: true, ReferencePrice: reference,
			EntrySlippageTolerancePercentage: decimal.RequireFromString("0.1"),
			OpenQuantity:                     decimal.NewNullDecimal(decimal.RequireFromString("0.002")),
			EntryLimitPrice:                  decimal.NewNullDecimal(decimal.RequireFromString("84069.9")),
			EntryStatus:                      string(vo.ContractEntryFilled),
			OpenedQuantity:                   decimal.NewNullDecimal(decimal.RequireFromString("0.002")),
			OpenAveragePrice:                 decimal.NewNullDecimal(decimal.NewFromInt(84100))},
		{RunNumber: 61, TargetPosition: "long", Status: string(vo.ContractAutoOrderSettled),
			Outcome: string(vo.ContractAutoOrderUnfilled), Reason: "進場單沒有成交，已撤單", ReferencePrice: reference,
			EntryLimitPrice: decimal.NewNullDecimal(decimal.RequireFromString("84238.1")),
			EntryStatus:     string(vo.ContractEntryUnfilled)},
		{RunNumber: 62, TargetPosition: "long", Status: string(vo.ContractAutoOrderReady), ReferencePrice: reference,
			EntryLimitPrice:       decimal.NewNullDecimal(decimal.RequireFromString("84238.1")),
			EntryExecutedQuantity: decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
			EntryStatus:           string(vo.ContractEntryWorking)},
	}

	runRecords, listError := underTest.strategyBotApplication.ListRunRecords(
		context.Background(), strategyBotOwnerID, strategyBotID, "")

	require.NoError(t, listError)
	require.Len(t, runRecords, 3)

	working := runRecords[0].AutoOrder
	require.NotNil(t, working)
	assert.Equal(t, "pending", working.Status)
	assert.Equal(t, "working", working.EntryStatus)
	assert.True(t, working.EntryExecutedQuantity.Equal(decimal.RequireFromString("0.001")))
	assert.Nil(t, working.EntrySlippagePercentage)

	unfilled := runRecords[1].AutoOrder
	require.NotNil(t, unfilled)
	assert.Equal(t, "unfilled", unfilled.Status)
	assert.Equal(t, "unfilled", unfilled.EntryStatus)
	assert.True(t, unfilled.EntryLimitPrice.Equal(decimal.RequireFromString("84238.1")))

	filled := runRecords[2].AutoOrder
	require.NotNil(t, filled)
	assert.Equal(t, "filled", filled.EntryStatus)
	assert.True(t, filled.ReferencePrice.Equal(decimal.NewFromInt(84154)))
	assert.True(t, filled.EntrySlippageTolerancePercentage.Equal(decimal.RequireFromString("0.1")))
	assert.True(t, filled.RequestedQuantity.Equal(decimal.RequireFromString("0.002")))
	require.NotNil(t, filled.EntrySlippagePercentage)
	// Selling 54 below the reference cost the owner 0.064%.
	assert.True(t, filled.EntrySlippagePercentage.Equal(decimal.RequireFromString("0.064")), "滑價 %s", filled.EntrySlippagePercentage)
}

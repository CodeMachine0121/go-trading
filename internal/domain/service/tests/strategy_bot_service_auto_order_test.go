package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// A round whose auto order cannot be queued is not booked at all, so the order is never silently lost while the round looks done.
func TestStrategyBotServiceRecordRoundFailsWhenItsAutoOrderCannotBeQueued(t *testing.T) {
	controller := gomock.NewController(t)
	dueAt := time.Date(2026, 10, 4, 6, 30, 0, 0, time.UTC)
	runningBot := entities.StrategyBot{
		ID: 3, OwnerID: 7, Name: "合約突破", Symbol: "BTCUSDT", RunState: string(vo.StrategyBotRunning),
		MarketDataKind: string(vo.MarketDataKindContractKCandle), AutoOrderEnabled: true,
		TriggerIntervalMinutes: 5, NextRunAt: dueAt,
	}

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	strategyBotRepository.EXPECT().FindOneLocked(gomock.Any(), uint(3)).Return(runningBot, nil)
	strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(controller)
	strategyBotRunRecordRepository.EXPECT().Append(gomock.Any(), gomock.Any()).Return(52, nil)
	pendingMessageRepository := mocks.NewMockIPendingMessageRepository(controller)
	pendingMessageRepository.EXPECT().Enqueue(gomock.Any(), gomock.Any()).Return(nil)
	queueFailure := errors.New("the database went away")
	contractAutoOrderRepository := mocks.NewMockIContractAutoOrderRepository(controller)
	contractAutoOrderRepository.EXPECT().Enqueue(gomock.Any(), gomock.Any()).Return(queueFailure)
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		})
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(dueAt.Add(time.Second)).AnyTimes()

	strategyBotService := service.NewStrategyBotService(
		strategyBotRepository, strategyBotRunRecordRepository, nil, nil, nil,
		pendingMessageRepository, contractAutoOrderRepository, transactionRepository, clockProxy)

	_, applied, recordError := strategyBotService.RecordRound(t.Context(), 3, dueAt, "replica-a",
		dto.StrategyBotRoundOutcomeDto{
			Kind: "concluded", Verdict: string(vo.SignalBuy), SentSignal: string(vo.SignalBuy),
			HasMessage: true, HasAutoOrderIntent: true,
			Round: dto.StrategyBotRoundDto{
				BotName: "合約突破", Symbol: "BTCUSDT", MarketDataKind: string(vo.MarketDataKindContractKCandle),
				Verdict: string(vo.SignalBuy),
			},
			AutoOrderIntent: dto.ContractAutoOrderIntentDto{
				TargetPosition: "long", OpenQuantity: decimal.RequireFromString("0.002"), HasOpenQuantity: true,
				Leverage: decimal.NewFromInt(3),
			},
		})

	require.ErrorIs(t, recordError, queueFailure)
	require.False(t, applied)
}

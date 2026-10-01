package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var autoOrderKeyConfiguredAt = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

func storedBotOfKind(runState vo.StrategyBotRunStateVo, marketDataKind vo.MarketDataKindVo) entities.StrategyBot {
	bot := storedBot(runState)
	bot.MarketDataKind = string(marketDataKind)

	return bot
}

func (underTest strategyBotApplicationUnderTest) theOwnerHasATradingKeyFor(spot bool, contract bool) {
	underTest.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.BinanceTradingKey{
			UserID: strategyBotOwnerID, ApiKeyTail: "a1b2",
			SpotTradingEnabled: spot, ContractTradingEnabled: contract,
			UpdatedAt: autoOrderKeyConfiguredAt,
		}, nil).AnyTimes()
}

func (underTest strategyBotApplicationUnderTest) theOwnerHasNoTradingKey() {
	underTest.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), gomock.Any()).
		Return(entities.BinanceTradingKey{}, domains.ErrBinanceTradingKeyNotConfigured).AnyTimes()
}

func TestStrategyBotApplicationEnableAutoOrder(t *testing.T) {
	t.Run("a stopped or running spot bot with a spot key is switched on and keeps its run state", func(t *testing.T) {
		for _, runState := range []vo.StrategyBotRunStateVo{vo.StrategyBotStopped, vo.StrategyBotRunning} {
			t.Run(string(runState), func(t *testing.T) {
				underTest := newStrategyBotApplicationUnderTest(t)
				underTest.theOwnerHasATradingKeyFor(true, false)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(storedBotOfKind(runState, vo.MarketDataKindKCandle), nil)
				underTest.strategyBotRepository.EXPECT().
					EnableAutoOrder(gomock.Any(), strategyBotID, strategyBotOwnerID, autoOrderKeyConfiguredAt).
					Return(nil)

				botDto, err := underTest.strategyBotApplication.EnableAutoOrder(
					context.Background(), strategyBotOwnerID, strategyBotID)

				require.NoError(t, err)
				assert.True(t, botDto.AutoOrderEnabled)
				assert.Equal(t, string(runState), botDto.RunState)
			})
		}
	})

	t.Run("a contract bot with a contract key is switched on", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.theOwnerHasATradingKeyFor(false, true)
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBotOfKind(vo.StrategyBotStopped, vo.MarketDataKindContractKCandle), nil)
		underTest.strategyBotRepository.EXPECT().
			EnableAutoOrder(gomock.Any(), strategyBotID, strategyBotOwnerID, autoOrderKeyConfiguredAt).
			Return(nil)

		botDto, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.NoError(t, err)
		assert.True(t, botDto.AutoOrderEnabled)
	})

	// The repository is not set up to write, so any write fails the test: the bot stays off.
	t.Run("without a trading key it is refused", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.theOwnerHasNoTradingKey()
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBot(vo.StrategyBotStopped), nil)

		_, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.ErrorIs(t, err, domains.ErrStrategyBotAutoOrderKeyNotConfigured)
		assert.Contains(t, err.Error(), "請先完成幣安交易金鑰設定")
	})

	t.Run("a contract bot with a spot-only key is refused", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.theOwnerHasATradingKeyFor(true, false)
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBotOfKind(vo.StrategyBotStopped, vo.MarketDataKindContractKCandle), nil)

		_, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.ErrorIs(t, err, domains.ErrStrategyBotAutoOrderMarketNotCovered)
		assert.Contains(t, err.Error(), "這組幣安交易金鑰沒有合約交易權限")
	})

	t.Run("an already-on bot is not a failure and is not written again", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.theOwnerHasATradingKeyFor(true, false)
		alreadyOn := storedBot(vo.StrategyBotStopped)
		alreadyOn.AutoOrderEnabled = true
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(alreadyOn, nil)

		botDto, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.NoError(t, err)
		assert.True(t, botDto.AutoOrderEnabled)
	})

	t.Run("a stranger's bot is not found", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), strategyBotStrangerID).
			Return(entities.BinanceTradingKey{
				UserID: strategyBotStrangerID, SpotTradingEnabled: true, UpdatedAt: autoOrderKeyConfiguredAt,
			}, nil)
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBot(vo.StrategyBotStopped), nil)

		_, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotStrangerID, strategyBotID)

		require.ErrorIs(t, err, domains.ErrStrategyBotNotFound)
	})

	t.Run("a key changed meanwhile is reported as such", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.theOwnerHasATradingKeyFor(true, true)
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBot(vo.StrategyBotStopped), nil)
		underTest.strategyBotRepository.EXPECT().
			EnableAutoOrder(gomock.Any(), strategyBotID, strategyBotOwnerID, autoOrderKeyConfiguredAt).
			Return(domains.ErrStrategyBotAutoOrderKeyChanged)

		_, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.ErrorIs(t, err, domains.ErrStrategyBotAutoOrderKeyChanged)
	})

	t.Run("a key that cannot be read is reported, not read as missing", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		storageFailure := errors.New("the database went away")
		underTest.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), strategyBotOwnerID).
			Return(entities.BinanceTradingKey{}, storageFailure)

		_, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.ErrorIs(t, err, storageFailure)
		assert.NotErrorIs(t, err, domains.ErrStrategyBotAutoOrderKeyNotConfigured)
	})

	t.Run("a bot that cannot be read is reported", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.theOwnerHasATradingKeyFor(true, true)
		storageFailure := errors.New("the database went away")
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(entities.StrategyBot{}, storageFailure)

		_, err := underTest.strategyBotApplication.EnableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestStrategyBotApplicationDisableAutoOrder(t *testing.T) {
	// No trading key is ever read: the key store is not set up, so reading it fails the test.
	t.Run("switching off needs no key and works whether it was on or off", func(t *testing.T) {
		for _, wasOn := range []bool{true, false} {
			underTest := newStrategyBotApplicationUnderTest(t)
			bot := storedBot(vo.StrategyBotRunning)
			bot.AutoOrderEnabled = wasOn
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(bot, nil)
			underTest.strategyBotRepository.EXPECT().DisableAutoOrder(gomock.Any(), strategyBotID).Return(nil)

			botDto, err := underTest.strategyBotApplication.DisableAutoOrder(
				context.Background(), strategyBotOwnerID, strategyBotID)

			require.NoError(t, err)
			assert.False(t, botDto.AutoOrderEnabled)
			assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
		}
	})

	t.Run("a stranger's bot is not found and is left alone", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBot(vo.StrategyBotStopped), nil)

		_, err := underTest.strategyBotApplication.DisableAutoOrder(
			context.Background(), strategyBotStrangerID, strategyBotID)

		require.ErrorIs(t, err, domains.ErrStrategyBotNotFound)
	})

	t.Run("a write that fails is reported", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		storageFailure := errors.New("the database went away")
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBot(vo.StrategyBotStopped), nil)
		underTest.strategyBotRepository.EXPECT().DisableAutoOrder(gomock.Any(), strategyBotID).
			Return(storageFailure)

		_, err := underTest.strategyBotApplication.DisableAutoOrder(
			context.Background(), strategyBotOwnerID, strategyBotID)

		require.ErrorIs(t, err, storageFailure)
	})
}

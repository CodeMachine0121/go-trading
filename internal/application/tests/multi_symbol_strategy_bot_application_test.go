package application_test

import (
	"context"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// aStoppedBotWatchingBitcoinAndEthereum last said buy on BTC; ETH holds whatever position is given.
func aStoppedBotWatchingBitcoinAndEthereum(ethereumPosition string) entities.StrategyBot {
	bot := storedBot(vo.StrategyBotStopped)
	bot.WatchedSymbols = []entities.StrategyBotWatchedSymbol{
		{StrategyBotID: strategyBotID, Symbol: "BTCUSDT", LastSentSignal: string(vo.SignalBuy)},
		{StrategyBotID: strategyBotID, Symbol: "ETHUSDT", AutoOrderPositionDirection: ethereumPosition},
	}

	return bot
}

func TestStrategyBotApplicationRewritesTheListOfWatchedSymbols(t *testing.T) {
	testCases := []struct {
		name            string
		ethereumHolds   string
		ethereumInWork  bool
		writtenSymbols  []string
		expectedSaved   []string
		expectedRefusal error
		expectedMessage string
	}{
		{name: "a symbol added beside the ones kept", writtenSymbols: []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"},
			expectedSaved: []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}},
		{name: "a flat symbol removed", writtenSymbols: []string{"BTCUSDT"}, expectedSaved: []string{"BTCUSDT"}},
		{name: "a symbol still holding a position cannot be removed", ethereumHolds: "long",
			writtenSymbols: []string{"BTCUSDT"}, expectedRefusal: domains.ErrStrategyBotWatchedSymbolHoldsPosition,
			expectedMessage: "ETHUSDT 還有機器人持倉，要先讓它空手才能移除"},
		{name: "a symbol with an auto order not yet done cannot be removed", ethereumInWork: true,
			writtenSymbols: []string{"BTCUSDT"}, expectedRefusal: domains.ErrStrategyBotWatchedSymbolAutoOrderOutstanding,
			expectedMessage: "ETHUSDT 還有沒做完的自動下單，要等它做完才能移除"},
		{name: "a symbol holding a position may stay", ethereumHolds: "long",
			writtenSymbols: []string{"ETHUSDT", "BTCUSDT"}, expectedSaved: []string{"ETHUSDT", "BTCUSDT"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.expectTheNamedTradingStrategyIsThisPersons()
			underTest.unsettledAutoOrderSymbols["ETHUSDT"] = testCase.ethereumInWork
			stored := aStoppedBotWatchingBitcoinAndEthereum(testCase.ethereumHolds)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stored, nil).AnyTimes()
			if testCase.expectedRefusal == nil {
				underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
						assert.Equal(t, testCase.expectedSaved, bot.Symbols())

						return bot, nil
					})
			}
			writeDto := aBotWrite()
			writeDto.ID = strategyBotID
			writeDto.Symbols = testCase.writtenSymbols

			_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			if testCase.expectedRefusal == nil {
				require.NoError(t, updateError)
				return
			}
			require.ErrorIs(t, updateError, testCase.expectedRefusal)
			assert.ErrorContains(t, updateError, testCase.expectedMessage)
		})
	}
}

func TestStrategyBotApplicationStartRevivesEveryWatchedSymbolButKeepsWhatAutoOrdersOpened(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	stopped := aStoppedBotWatchingBitcoinAndEthereum("long")
	stopped.WatchedSymbols[1].AutoOrderPositionQuantity = decimal.RequireFromString("0.5")
	stopped.WatchedSymbols[1].HaltReason = string(vo.StrategyBotHaltScriptFailed)
	stopped.WatchedSymbols[1].Conflicting = true
	underTest.telegramDeliveryRepository.EXPECT().FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stopped, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(0, nil)
	started := entities.StrategyBot{}
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			started = bot

			return nil
		})

	_, startError := underTest.strategyBotApplication.StartStrategyBot(context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, startError)
	for _, watchedSymbol := range started.WatchedSymbols {
		assert.Empty(t, watchedSymbol.LastSentSignal, watchedSymbol.Symbol)
		assert.Empty(t, watchedSymbol.HaltReason, watchedSymbol.Symbol)
		assert.False(t, watchedSymbol.Conflicting, watchedSymbol.Symbol)
	}
	ethereum, _ := started.WatchedSymbol("ETHUSDT")
	assert.Equal(t, "long", ethereum.AutoOrderPositionDirection)
	assert.True(t, decimal.RequireFromString("0.5").Equal(ethereum.AutoOrderPositionQuantity))
	require.Len(t, underTest.announced(), 1)
	assert.Contains(t, underTest.announced()[0], "【已啟動】早盤突破 · BTCUSDT、ETHUSDT")
	assert.Contains(t, underTest.announced()[0], "它開始盯這 2 檔了")
}

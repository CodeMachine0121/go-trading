package application_test

import (
	"context"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// runsTheContractRoundOf runs one scheduled round of this bot at 85000 with its two sources saying these, and hands back the bot as the round left it.
func (underTest strategyBotRunUnderTest) runsTheContractRoundOf(
	t *testing.T, bot entities.StrategyBot, firstSourceSignal vo.SignalVo, secondSourceSignal vo.SignalVo,
) entities.StrategyBot {
	underTest.makeTheRulesContract()
	underTest.expectContractSources(firstSourceSignal, secondSourceSignal)
	underTest.expectTheContractsLatestCandle("85000")
	underTest.strategyBotRepository.EXPECT().ClaimDue(gomock.Any(), botRunNow, 4, thisReplicaName, botRoundClaimedUntil).
		Return([]entities.StrategyBot{bot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(bot, nil).AnyTimes()
	leftBot := entities.StrategyBot{}
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, updated entities.StrategyBot) error {
			leftBot = updated
			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)

	return leftBot
}

func TestASilencedBuyIsStillDecidedAndTraded(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectTheVenue(aContractSpecification(), nil, "")
	underTest.expectNoRoundMessage()
	bot := anAutoOrderingContractBot("")
	bot.ContractTradingMode = string(vo.ContractTradingModeLongOnly)
	bot.MutesBuyNotification = true

	leftBot := underTest.runsTheContractRoundOf(t, bot, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, *underTest.queuedAutoOrders, 1)
	assert.Equal(t, "long", (*underTest.queuedAutoOrders)[0].TargetPosition)
	require.Len(t, leftBot.WatchedSymbols, 1)
	assert.Equal(t, "buy", leftBot.WatchedSymbols[0].LastSentSignal)
	assert.Equal(t, "buy", leftBot.WatchedSymbols[0].LastOpinion)
}

func TestASilencedSignalStillHoldsBackItsRepeat(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectNoRoundMessage()
	bot := anAutoOrderingContractBot(string(vo.SignalBuy))
	bot.ContractTradingMode = string(vo.ContractTradingModeLongOnly)
	bot.MutesBuyNotification = true

	underTest.runsTheContractRoundOf(t, bot, vo.SignalBuy, vo.SignalBuy)

	assert.Empty(t, *underTest.queuedAutoOrders)
}

func TestReturningToHoldIsSaidOnce(t *testing.T) {
	testCases := []struct {
		name          string
		sendsHold     bool
		lastOpinion   string
		expectMessage bool
	}{
		{name: "back to holding after a buy", sendsHold: true, lastOpinion: "buy", expectMessage: true},
		{name: "first conclusion after a start", sendsHold: true, lastOpinion: "", expectMessage: true},
		{name: "still holding", sendsHold: true, lastOpinion: "hold", expectMessage: false},
		{name: "hold notification off", sendsHold: false, lastOpinion: "buy", expectMessage: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			if testCase.expectMessage {
				underTest.expectQueuedMessage(func(message string) {
					assert.Contains(t, message, "【回到持有】合約突破 · BTCUSDT 永續合約")
				})
			} else {
				underTest.expectNoRoundMessage()
			}
			bot := anAutoOrderingContractBot(string(vo.SignalBuy))
			bot.SendsHoldNotification = testCase.sendsHold
			bot.WatchedSymbols[0].LastOpinion = testCase.lastOpinion

			leftBot := underTest.runsTheContractRoundOf(t, bot, vo.SignalHold, vo.SignalHold)

			assert.Empty(t, *underTest.queuedAutoOrders, "holding never trades")
			require.Len(t, leftBot.WatchedSymbols, 1)
			assert.Equal(t, "hold", leftBot.WatchedSymbols[0].LastOpinion)
			assert.Equal(t, "buy", leftBot.WatchedSymbols[0].LastSentSignal, "holding never moves the last sent signal")
			if testCase.expectMessage {
				require.Len(t, underTest.queuedMessages(), 1)
				assert.Equal(t, "hold", underTest.queuedMessages()[0].Signal)
			}
		})
	}
}

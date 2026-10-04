package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// anAutoOrderingContractBot is a running contract bot with auto order on and a plan whose
// suggestion at 85000 is 0.002: 60 of margin at 3 times leverage is 180 of notional,
// stepped down to the contract's 0.001.
func anAutoOrderingContractBot(lastSentSignal string) entities.StrategyBot {
	bot := aDueContractBot(lastSentSignal)
	bot.AutoOrderEnabled = true
	bot.PositionPlanCapital = decimal.NewFromInt(60)
	bot.PositionPlanLeverage = decimal.NewFromInt(3)
	bot.PositionPlanStopLossPercentage = decimal.RequireFromString("1.5")
	bot.PositionPlanTakeProfitPercentage = decimal.NewFromInt(3)

	return bot
}

// runsAContractRound runs one scheduled round of this bot at 85000, its conclusion set by the
// two sources, and expects it to say something.
func (underTest strategyBotRunUnderTest) runsAContractRound(
	t *testing.T, bot entities.StrategyBot, tradingMode vo.ContractTradingModeVo,
	firstSourceSignal vo.SignalVo, secondSourceSignal vo.SignalVo,
) {
	underTest.makeTheRulesContract(tradingMode)
	underTest.expectContractSources(firstSourceSignal, secondSourceSignal)
	underTest.expectTheContractsLatestCandle("85000")
	underTest.strategyBotRepository.EXPECT().ClaimDue(gomock.Any(), botRunNow, 4, thisReplicaName, botRoundClaimedUntil).
		Return([]entities.StrategyBot{bot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(bot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	underTest.expectQueuedMessage(func(string) {})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestARoundThatChangesItsConclusionQueuesAnAutoOrderFromItsOwnPlan(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectTheVenue(aContractSpecification(), nil, "")

	underTest.runsAContractRound(t, anAutoOrderingContractBot(string(vo.SignalSell)),
		vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, *underTest.queuedAutoOrders, 1)
	queued := (*underTest.queuedAutoOrders)[0]
	assert.Equal(t, strategyBotID, queued.StrategyBotID)
	assert.Equal(t, strategyBotOwnerID, queued.OwnerUserID)
	assert.Equal(t, "BTCUSDT", queued.Symbol)
	assert.Equal(t, "long", queued.TargetPosition)
	require.True(t, queued.OpenQuantity.Valid)
	assert.True(t, queued.OpenQuantity.Decimal.Equal(decimal.RequireFromString("0.002")), "開倉數量 %s", queued.OpenQuantity.Decimal)
	assert.True(t, queued.Leverage.Equal(decimal.NewFromInt(3)))
	assert.True(t, queued.StopLossPercentage.Equal(decimal.RequireFromString("1.5")))
	assert.True(t, queued.TakeProfitPercentage.Equal(decimal.NewFromInt(3)))
	assert.Equal(t, botRunNow.Add(2*time.Minute), queued.ExpiresAt)
	assert.Equal(t, string(vo.ContractAutoOrderReady), queued.Status)
	assert.Equal(t, bookedRunNumber, queued.RunNumber)
}

func TestARoundThatRepeatsItsConclusionQueuesNoAutoOrder(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongOnly)
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
	underTest.expectTheContractsLatestCandle("85000")
	bot := anAutoOrderingContractBot(string(vo.SignalBuy))
	underTest.strategyBotRepository.EXPECT().ClaimDue(gomock.Any(), botRunNow, 4, thisReplicaName, botRoundClaimedUntil).
		Return([]entities.StrategyBot{bot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(bot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	assert.Empty(t, *underTest.queuedAutoOrders)
}

func TestABotWithAutoOrderOffOnlySpeaks(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectTheVenue(aContractSpecification(), nil, "")
	bot := anAutoOrderingContractBot(string(vo.SignalSell))
	bot.AutoOrderEnabled = false

	underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	assert.Len(t, underTest.queuedMessages(), 1)
	assert.Empty(t, *underTest.queuedAutoOrders)
}

func TestAStoppedBotRunByHandNeverOrders(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongOnly)
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
	underTest.expectTheContractsLatestCandle("85000")
	underTest.expectTheVenue(aContractSpecification(), nil, "")
	stoppedBot := anAutoOrderingContractBot(string(vo.SignalSell))
	stoppedBot.RunState = string(vo.StrategyBotStopped)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stoppedBot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	underTest.expectQueuedMessage(func(string) {})

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, runError)
	assert.Empty(t, *underTest.queuedAutoOrders)
}

func TestARunningBotRunByHandOrdersLikeAScheduledRound(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongOnly)
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
	underTest.expectTheContractsLatestCandle("85000")
	underTest.expectTheVenue(aContractSpecification(), nil, "")
	runningBot := anAutoOrderingContractBot(string(vo.SignalSell))
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(runningBot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	underTest.expectQueuedMessage(func(string) {})

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, runError)
	require.Len(t, *underTest.queuedAutoOrders, 1)
	queued := (*underTest.queuedAutoOrders)[0]
	assert.Equal(t, "long", queued.TargetPosition)
	assert.True(t, queued.OpenQuantity.Decimal.Equal(decimal.RequireFromString("0.002")))
}

func TestABotWithoutAPositionPlanQueuesAnOrderThatOpensNothing(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	bot := anAutoOrderingContractBot(string(vo.SignalSell))
	bot.PositionPlanCapital = decimal.Zero

	underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, *underTest.queuedAutoOrders, 1)
	queued := (*underTest.queuedAutoOrders)[0]
	assert.Equal(t, "long", queued.TargetPosition)
	assert.False(t, queued.OpenQuantity.Valid)
	assert.Equal(t, "沒有部位規劃，不下單", queued.NoOpenReason)
}

func TestARoundTheVenueWouldRefuseQueuesAnOrderThatOpensNothing(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectTheVenue(aContractSpecification(), nil, "")
	bot := anAutoOrderingContractBot(string(vo.SignalSell))
	bot.PositionPlanCapital = decimal.NewFromInt(1)
	bot.PositionPlanLeverage = decimal.NewFromInt(1)

	underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, *underTest.queuedAutoOrders, 1)
	queued := (*underTest.queuedAutoOrders)[0]
	assert.False(t, queued.OpenQuantity.Valid)
	assert.True(t, strings.HasPrefix(queued.NoOpenReason, "交易所不收這一筆"), queued.NoOpenReason)
}

func TestTheTargetOfAQueuedOrderFollowsTheTradingMode(t *testing.T) {
	testCases := []struct {
		name           string
		tradingMode    vo.ContractTradingModeVo
		lastSent       vo.SignalVo
		expectedTarget string
		opensSomething bool
	}{
		{name: "只做多時賣出是平多", tradingMode: vo.ContractTradingModeLongOnly, lastSent: vo.SignalBuy,
			expectedTarget: "flat"},
		{name: "只做空時賣出是做空", tradingMode: vo.ContractTradingModeShortOnly, lastSent: vo.SignalBuy,
			expectedTarget: "short", opensSomething: true},
		{name: "多空反手時賣出是做空", tradingMode: vo.ContractTradingModeLongShort, lastSent: vo.SignalBuy,
			expectedTarget: "short", opensSomething: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			if testCase.opensSomething {
				underTest.expectTheVenue(aContractSpecification(), nil, "")
			}

			underTest.runsAContractRound(t, anAutoOrderingContractBot(string(testCase.lastSent)),
				testCase.tradingMode, vo.SignalSell, vo.SignalSell)

			require.Len(t, *underTest.queuedAutoOrders, 1)
			queued := (*underTest.queuedAutoOrders)[0]
			assert.Equal(t, testCase.expectedTarget, queued.TargetPosition)
			assert.Equal(t, testCase.opensSomething, queued.OpenQuantity.Valid)
			if !testCase.opensSomething {
				assert.Empty(t, queued.NoOpenReason)
			}
		})
	}
}

func TestARoundWhoseTradingModeCannotBeReadQueuesNoAutoOrder(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.runsAContractRound(t, anAutoOrderingContractBot(string(vo.SignalSell)),
		vo.ContractTradingModeVo("sideways"), vo.SignalBuy, vo.SignalBuy)

	assert.Empty(t, *underTest.queuedAutoOrders)
}

func TestABotLeftAtNoLeverageQueuesOrdersAtOneTimes(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	bot := anAutoOrderingContractBot(string(vo.SignalSell))
	bot.PositionPlanCapital = decimal.Zero
	bot.PositionPlanLeverage = decimal.Zero

	underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

	require.Len(t, *underTest.queuedAutoOrders, 1)
	assert.True(t, (*underTest.queuedAutoOrders)[0].Leverage.Equal(decimal.NewFromInt(1)))
}

func TestARoundThatCannotOpenSaysWhyInTheMessagesWords(t *testing.T) {
	smallestNotional := aContractSpecification()
	smallestNotional.MinimumNotional = decimal.NewNullDecimal(decimal.NewFromInt(200))
	twoTimesAtMost := []entities.ContractMaintenanceMarginTier{{
		Symbol: "BTCUSDT", Tier: 1, NotionalCap: decimal.NewFromInt(1000000),
		MaintenanceMarginRate: decimal.RequireFromString("0.004"), MaximumLeverage: 2,
	}}

	testCases := []struct {
		name           string
		venue          func(underTest strategyBotRunUnderTest)
		shapeTheBot    func(bot *entities.StrategyBot)
		expectedReason string
		reasonPrefix   bool
	}{
		{name: "部位資金不足",
			venue: func(strategyBotRunUnderTest) {},
			shapeTheBot: func(bot *entities.StrategyBot) {
				bot.PositionPlanCapital = decimal.NewFromInt(1000)
				bot.PositionPlanSizingMode = string(vo.PositionSizingModeFixedAmount)
				bot.PositionPlanSizingValue = decimal.NewFromInt(2000)
			},
			expectedReason: "部位資金不足，押不下 2000，不下單"},
		{name: "讀不到交易規格",
			venue: func(underTest strategyBotRunUnderTest) {
				underTest.expectTheVenue(entities.ContractTradingSymbol{}, nil, "")
			},
			shapeTheBot:    func(*entities.StrategyBot) {},
			expectedReason: "這個合約標的還沒有交易規格，算不出下單數量，不下單"},
		{name: "名目低於最小名目",
			venue: func(underTest strategyBotRunUnderTest) {
				underTest.expectTheVenue(smallestNotional, nil, "")
			},
			shapeTheBot:    func(*entities.StrategyBot) {},
			expectedReason: "交易所不收這一筆：名目 170 低於最小名目 200"},
		{name: "那一級最高槓桿",
			venue: func(underTest strategyBotRunUnderTest) {
				underTest.expectTheVenue(aContractSpecification(), twoTimesAtMost, "")
			},
			shapeTheBot:    func(*entities.StrategyBot) {},
			expectedReason: "交易所不收這一筆：名目 170 那一級最高只能開 2 倍"},
		{name: "倉位大小模式讀不懂",
			venue: func(strategyBotRunUnderTest) {},
			shapeTheBot: func(bot *entities.StrategyBot) {
				bot.PositionPlanSizingMode = "everything"
			},
			expectedReason: "這一輪算不出建議部位，不下單"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			testCase.venue(underTest)
			bot := anAutoOrderingContractBot(string(vo.SignalSell))
			testCase.shapeTheBot(&bot)

			underTest.runsAContractRound(t, bot, vo.ContractTradingModeLongOnly, vo.SignalBuy, vo.SignalBuy)

			require.Len(t, *underTest.queuedAutoOrders, 1)
			queued := (*underTest.queuedAutoOrders)[0]
			assert.False(t, queued.OpenQuantity.Valid)
			assert.Equal(t, testCase.expectedReason, queued.NoOpenReason)
		})
	}
}

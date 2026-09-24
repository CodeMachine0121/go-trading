package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// aDueContractBot is a running contract bot, due now, following the rules the fixture
// holds — turned into a contract trading strategy by makeTheRulesContract.
func aDueContractBot(lastSentSignal string) entities.StrategyBot {
	bot := aDueBot(lastSentSignal)
	bot.Name = "合約突破"
	bot.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	bot.PositionPlanLeverage = decimal.NewFromInt(1)

	return bot
}

// makeTheRulesContract turns the fixture's rules into a contract trading strategy
// trading by this mode, keeping its two sources and both conditions.
func (underTest strategyBotRunUnderTest) makeTheRulesContract(tradingMode vo.ContractTradingModeVo) {
	underTest.tradingStrategy.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	underTest.tradingStrategy.TradingMode = string(tradingMode)
}

// expectContractSources makes both strategy scripts resolvable as contract scripts and
// has each one's script say its own signal over contract bars — and only over contract
// bars: the spot runner has no expectation, so reaching it fails the test.
func (underTest strategyBotRunUnderTest) expectContractSources(
	firstSourceSignal vo.SignalVo, secondSourceSignal vo.SignalVo,
) {
	signalsByScript := map[string]vo.SignalVo{
		scriptOfStrategyScript(9):  firstSourceSignal,
		scriptOfStrategyScript(10): secondSourceSignal,
	}

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID, Script: scriptOfStrategyScript(id),
				ResultType: "signal", MarketDataKind: string(vo.MarketDataKindContractKCandle),
			}, nil
		}).AnyTimes()

	underTest.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return(storedContractCandlesAt(at(9, 10)), nil).AnyTimes()

	underTest.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, script string, _ domains.IndicatorResultTypeDomain,
			_ []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			return map[string]vo.IndicatorValueVo{
				vo.SignalIndicatorKey: {Signal: signalsByScript[script]},
			}, nil
		}).AnyTimes()
}

// expectTheContractsLatestCandle is the newest one-minute contract candle, closing at
// this price.
func (underTest strategyBotRunUnderTest) expectTheContractsLatestCandle(closePrice string) {
	latestCandle := storedContractCandlesAt(at(9, 14))[0]
	latestCandle.Close = decimal.RequireFromString(closePrice)

	underTest.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandleContract{latestCandle}, nil)
}

func TestStrategyBotRunApplicationRunsAContractBotOverContractBars(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
	underTest.expectTheContractsLatestCandle("64000.5")

	dueBot := aDueContractBot(string(vo.SignalSell))
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()

	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "🟢【做多】合約突破 · BTCUSDT 永續合約")
			assert.Contains(t, message, "⚙️ 交易模式 多空反手")
			assert.Contains(t, message, "💰 參考價 64000.5")
			assert.Contains(t, message, "那一根一分鐘合約 K 線的收盤價")

			return vo.DeliveryFailureNone, nil
		})

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.SignalBuy), bot.LastSentSignal)
			assert.Equal(t, botRunNow.Add(5*time.Minute), bot.NextRunAt)

			return nil
		})

	roundsRun, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	assert.Equal(t, 1, roundsRun)
}

func TestStrategyBotRunApplicationDoesNotRepeatAContractConclusion(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)

	dueBot := aDueContractBot(string(vo.SignalBuy))
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil)
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

// A contract taken off the watchlist stops sending bars. That comes right on its own
// if it is put back, so the round is skipped and the bot keeps running.
func TestStrategyBotRunApplicationKeepsAContractBotRunningWhenItsBarsStopArriving(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID, Script: scriptOfStrategyScript(id), ResultType: "signal",
				MarketDataKind: string(vo.MarketDataKindContractKCandle),
			}, nil
		}).AnyTimes()
	underTest.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.expectNoRoundMessage()

	dueBot := aDueContractBot("")
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotRunning), bot.RunState)
			assert.Empty(t, bot.HaltReason)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

// A contract bot halts for the same reasons a spot bot does, in the same words.
func TestStrategyBotRunApplicationHaltsAContractBotWhoseScriptIsGone(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(9)).AnyTimes()

	dueBot := aDueContractBot("")
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "【已停擺】合約突破 · BTCUSDT 永續合約")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
			assert.Equal(t, string(vo.StrategyBotHaltStrategyScriptUnavailable), bot.HaltReason)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

// The figures a contract round suggests reach the message and the history alike: a
// thousand staked whole at five times, a two percent stop, from a price of a hundred.
func TestStrategyBotRunApplicationSuggestsAContractPositionAndRemembersIt(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
	underTest.expectTheContractsLatestCandle("100")

	dueBot := aDueContractBot("")
	dueBot.PositionPlanCapital = decimal.NewFromInt(1000)
	dueBot.PositionPlanStopLossPercentage = decimal.NewFromInt(2)
	dueBot.PositionPlanLeverage = decimal.NewFromInt(5)
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "保證金 1000（5 倍槓桿，名目 5000）")
			assert.Contains(t, message, "止損 98（往下，虧 100）")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	underTest.strategyBotRunApplication.RunDueRounds(t.Context())

	require.Len(t, *underTest.appendedRunRecords, 1)
	recorded := (*underTest.appendedRunRecords)[0]
	require.True(t, recorded.HasPositionPlan)
	assert.Equal(t, "1000", recorded.PositionPlan.Stake.String())
	assert.Equal(t, "98", recorded.PositionPlan.StopLossPrice.String())
}

// A contract whose newest candle cannot be read still gets its message, saying so.
func TestStrategyBotRunApplicationStillSendsAContractRoundWithNoPriceToQuote(t *testing.T) {
	testCases := []struct {
		name          string
		storedCandles []entities.KCandleContract
		readError     error
	}{
		{name: "nothing stored", storedCandles: nil},
		{name: "the read failed", readError: errors.New("the database went away")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
			underTest.expectDeliverySetting()
			underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
			underTest.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
				Return(testCase.storedCandles, testCase.readError)

			dueBot := aDueContractBot("")
			underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
				Return([]entities.StrategyBot{dueBot}, nil)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
				Return(dueBot, nil).AnyTimes()
			underTest.messageDeliveryProxy.EXPECT().
				Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(
					_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
				) (vo.DeliveryFailureReasonVo, error) {
					assert.Contains(t, message, "【做多】")
					assert.Contains(t, message, "目前讀不到這個交易標的的最新合約 K 線")

					return vo.DeliveryFailureNone, nil
				})
			underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

			_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

			require.NoError(t, runError)
		})
	}
}

// What a sell means is read from the rules this round followed: under long only it
// closes the long, and there is nothing to suggest opening.
func TestStrategyBotRunApplicationReadsAContractConclusionByTheRulesTradingMode(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongOnly)
	underTest.expectDeliverySetting()
	underTest.expectContractSources(vo.SignalSell, vo.SignalHold)
	underTest.expectTheContractsLatestCandle("100")

	dueBot := aDueContractBot(string(vo.SignalBuy))
	dueBot.PositionPlanCapital = decimal.NewFromInt(1000)
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "⚪【平多】合約突破 · BTCUSDT 永續合約")
			assert.Contains(t, message, "⚙️ 交易模式 只做多")
			assert.NotContains(t, message, "建議部位")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	require.Len(t, *underTest.appendedRunRecords, 1)
	assert.False(t, (*underTest.appendedRunRecords)[0].HasPositionPlan)
}

// A contract taken off the watchlist leaves its old bars behind. A round that finds
// only those is skipped — it says nothing, keeps what it last sent, and keeps running —
// rather than concluding about the past as though it were now.
func TestStrategyBotRunApplicationSkipsAContractRoundJudgedByBarsNoLongerArriving(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID, Script: scriptOfStrategyScript(id), ResultType: "signal",
				MarketDataKind: string(vo.MarketDataKindContractKCandle),
			}, nil
		}).AnyTimes()
	// Three hours before the round, and nothing since.
	underTest.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return(storedContractCandlesAt(at(6, 0)), nil).AnyTimes()
	underTest.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}}, nil).AnyTimes()
	underTest.expectNoRoundMessage()

	dueBot := aDueContractBot(string(vo.SignalSell))
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotRunning), bot.RunState)
			assert.Empty(t, bot.HaltReason)
			assert.Equal(t, string(vo.SignalSell), bot.LastSentSignal)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	require.Len(t, *underTest.appendedRunRecords, 1)
	assert.Equal(t, string(vo.StrategyBotRoundResultHold), (*underTest.appendedRunRecords)[0].Result)
}

// A contract round reads no more than a contract calculation does: the bars once per
// source, and the newest candle once for the reference price.
func TestStrategyBotRunApplicationReadsTheContractMarketOncePerSource(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID, Script: scriptOfStrategyScript(id), ResultType: "signal",
				MarketDataKind: string(vo.MarketDataKindContractKCandle),
			}, nil
		}).Times(2)
	underTest.kCandleContractRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return(storedContractCandlesAt(at(9, 10)), nil).Times(2)
	underTest.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{vo.SignalIndicatorKey: {Signal: vo.SignalBuy}}, nil).Times(2)
	underTest.expectTheContractsLatestCandle("100")
	underTest.messageDeliveryProxy.EXPECT().Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.DeliveryFailureNone, nil)

	dueBot := aDueContractBot("")
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

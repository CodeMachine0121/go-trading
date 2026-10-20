package application_test

import (
	"context"
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

// aDueBotWatching is the due spot bot watching each symbol, every one having last said what is given for it.
func aDueBotWatching(lastSentSignals ...[2]string) entities.StrategyBot {
	bot := aDueBot("")
	bot.WatchedSymbols = []entities.StrategyBotWatchedSymbol{}
	for _, symbolAndSignal := range lastSentSignals {
		bot.WatchedSymbols = append(bot.WatchedSymbols, entities.StrategyBotWatchedSymbol{
			StrategyBotID: strategyBotID, Symbol: symbolAndSignal[0], LastSentSignal: symbolAndSignal[1],
		})
	}

	return bot
}

// sourcesBySymbol is what sources A and B say on each symbol this round; a symbol given failsOn has its script fail.
type sourcesBySymbol map[string][2]vo.SignalVo

// expectSourcesOn answers each symbol's candles with that symbol's own bars, so each symbol's sources speak for it alone.
func (underTest strategyBotRunUnderTest) expectSourcesOn(signals sourcesBySymbol, failingSymbols ...string) {
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategyScript(id), ResultType: "signal",
			}, nil
		}).AnyTimes()

	for symbol := range signals {
		candle := kCandleAt(at(9, 10), "100")
		candle.Symbol = symbol
		underTest.kCandleRepository.EXPECT().
			FindLatestBefore(gomock.Any(), symbol, gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{candle}, nil).AnyTimes()
		underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), symbol, 1).
			Return([]entities.KCandle{candle}, nil).AnyTimes()
	}

	underTest.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, script string, _ domains.IndicatorResultTypeDomain,
			candles []vo.KCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			symbol := candles[len(candles)-1].Symbol
			for _, failingSymbol := range failingSymbols {
				if symbol == failingSymbol {
					return nil, domains.ErrIndicatorScriptFailed
				}
			}
			sourceIndex := 0
			if script == scriptOfStrategyScript(10) {
				sourceIndex = 1
			}

			return map[string]vo.IndicatorValueVo{
				vo.SignalIndicatorKey: {Signal: signals[symbol][sourceIndex]},
			}, nil
		}).AnyTimes()
}

// runsTheBot runs one scheduled round of the bot and hands back what it stored.
func (underTest strategyBotRunUnderTest) runsTheBot(t *testing.T, bot entities.StrategyBot) entities.StrategyBot {
	underTest.strategyBotRepository.EXPECT().ClaimDue(gomock.Any(), botRunNow, 4, thisReplicaName, botRoundClaimedUntil).
		Return([]entities.StrategyBot{bot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(bot, nil).AnyTimes()
	storedBot := entities.StrategyBot{}
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, endedBot entities.StrategyBot) error {
			storedBot = endedBot

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())
	require.NoError(t, runError)

	return storedBot
}

func lastSentSignalOf(bot entities.StrategyBot, symbol string) string {
	watchedSymbol, _ := bot.WatchedSymbol(symbol)

	return watchedSymbol.LastSentSignal
}

func TestAMultiSymbolBotSpeaksOnlyForTheSymbolWhoseSignalChanged(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSourcesOn(sourcesBySymbol{
		"BTCUSDT": {vo.SignalBuy, vo.SignalBuy},
		"ETHUSDT": {vo.SignalSell, vo.SignalHold},
	})
	underTest.expectQueuedMessage(func(message string) {
		assert.Contains(t, message, "【出場】早盤突破 · ETHUSDT")
	})

	storedBot := underTest.runsTheBot(t, aDueBotWatching(
		[2]string{"BTCUSDT", string(vo.SignalBuy)}, [2]string{"ETHUSDT", string(vo.SignalBuy)}))

	assert.Equal(t, string(vo.SignalBuy), lastSentSignalOf(storedBot, "BTCUSDT"))
	assert.Equal(t, string(vo.SignalSell), lastSentSignalOf(storedBot, "ETHUSDT"))
	require.Len(t, *underTest.appendedRunRecords, 2)
	assert.Equal(t, "BTCUSDT", (*underTest.appendedRunRecords)[0].Symbol)
	assert.Equal(t, "buy", (*underTest.appendedRunRecords)[0].Result)
	assert.Equal(t, "ETHUSDT", (*underTest.appendedRunRecords)[1].Symbol)
	assert.Equal(t, "sell", (*underTest.appendedRunRecords)[1].Result)
	queued := underTest.queuedMessages()
	require.Len(t, queued, 1)
	assert.Equal(t, "ETHUSDT", queued[0].Symbol)
}

func TestAMultiSymbolBotSendsOneMessagePerSymbolThatConcludedAnew(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSourcesOn(sourcesBySymbol{
		"BTCUSDT": {vo.SignalBuy, vo.SignalBuy},
		"ETHUSDT": {vo.SignalBuy, vo.SignalBuy},
	})
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "【買入】早盤突破 · BTCUSDT") })
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "【買入】早盤突破 · ETHUSDT") })

	storedBot := underTest.runsTheBot(t, aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""}))

	assert.Equal(t, string(vo.SignalBuy), lastSentSignalOf(storedBot, "BTCUSDT"))
	assert.Equal(t, string(vo.SignalBuy), lastSentSignalOf(storedBot, "ETHUSDT"))
}

func TestAMultiSymbolBotMarksAConflictOnThatSymbolAlone(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	// Buy on B and sell on A, so a symbol where both hold conflicts.
	underTest.tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 10, TradingStrategyID: botsTradingStrategyID, Side: "buy", SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, TradingStrategyID: botsTradingStrategyID, Side: "sell", SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
	}
	underTest.expectSourcesOn(sourcesBySymbol{
		"BTCUSDT": {vo.SignalSell, vo.SignalBuy},
		"ETHUSDT": {vo.SignalHold, vo.SignalBuy},
	})
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "【買入】早盤突破 · ETHUSDT") })

	storedBot := underTest.runsTheBot(t, aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""}))

	bitcoin, _ := storedBot.WatchedSymbol("BTCUSDT")
	ethereum, _ := storedBot.WatchedSymbol("ETHUSDT")
	assert.True(t, bitcoin.Conflicting)
	assert.Empty(t, bitcoin.LastSentSignal)
	assert.False(t, ethereum.Conflicting)
	assert.Equal(t, string(vo.SignalBuy), ethereum.LastSentSignal)
	assert.Equal(t, string(vo.StrategyBotRunning), storedBot.RunState)
}

func TestAMultiSymbolBotSizesEverySymbolWithTheWholePlan(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSourcesOn(sourcesBySymbol{
		"BTCUSDT": {vo.SignalBuy, vo.SignalBuy},
		"ETHUSDT": {vo.SignalBuy, vo.SignalBuy},
	})
	for range 2 {
		underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "開倉金額 1000") })
	}
	bot := aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""})
	bot.PositionPlanCapital = decimal.NewFromInt(1000)
	bot.PositionPlanSizingMode = string(vo.PositionSizingModeFixedAmount)
	bot.PositionPlanSizingValue = decimal.NewFromInt(1000)

	underTest.runsTheBot(t, bot)

	require.Len(t, *underTest.appendedRunRecords, 2)
	for _, appended := range *underTest.appendedRunRecords {
		assert.True(t, appended.PositionPlan.Stake.Equal(decimal.NewFromInt(1000)), appended.Symbol)
	}
}

func TestAMultiSymbolBotHaltsOnlyTheSymbolWhoseScriptWillNotRunOnIt(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSourcesOn(sourcesBySymbol{
		"BTCUSDT": {vo.SignalBuy, vo.SignalBuy},
		"ETHUSDT": {vo.SignalBuy, vo.SignalBuy},
	}, "ETHUSDT")
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "【買入】早盤突破 · BTCUSDT") })
	underTest.expectQueuedMessage(func(message string) {
		assert.Contains(t, message, "【標的停擺】早盤突破 · ETHUSDT")
		assert.Contains(t, message, "原因：它用到的某一支策略腳本算不出來。")
		assert.Contains(t, message, "其他標的照常")
	})

	storedBot := underTest.runsTheBot(t, aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""}))

	assert.Equal(t, string(vo.StrategyBotRunning), storedBot.RunState)
	assert.Empty(t, storedBot.HaltReason)
	ethereum, _ := storedBot.WatchedSymbol("ETHUSDT")
	assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), ethereum.HaltReason)
	assert.Equal(t, botRunNow.Add(5*time.Minute), storedBot.NextRunAt)
	require.Len(t, *underTest.appendedRunRecords, 2)
	assert.Equal(t, "hold", (*underTest.appendedRunRecords)[1].Result)
}

func TestABotHaltsOnceItsLastWatchedSymbolHalts(t *testing.T) {
	testCases := []struct {
		name string
		bot  entities.StrategyBot
	}{
		{name: "a bot watching one symbol halts exactly as before",
			bot: aDueBotWatching([2]string{"BTCUSDT", ""})},
		{name: "a bot whose other symbol already halted", bot: func() entities.StrategyBot {
			bot := aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""})
			bot.WatchedSymbols[1].HaltReason = string(vo.StrategyBotHaltOrderFlowRefused)

			return bot
		}()},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			// ETH's sources are deliberately absent: a halted symbol is never played.
			underTest.expectSourcesOn(sourcesBySymbol{"BTCUSDT": {vo.SignalBuy, vo.SignalBuy}}, "BTCUSDT")
			underTest.expectQueuedMessage(func(message string) {
				assert.Contains(t, message, "【已停擺】早盤突破")
				assert.Contains(t, message, "原因：它用到的某一支策略腳本算不出來。")
				assert.NotContains(t, message, "【標的停擺】")
			})

			storedBot := underTest.runsTheBot(t, testCase.bot)

			assert.Equal(t, string(vo.StrategyBotStopped), storedBot.RunState)
			assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), storedBot.HaltReason)
			assert.Len(t, *underTest.appendedRunRecords, 1)
		})
	}
}

func TestAMultiSymbolBotHaltsWholeWhenItsTradingStrategyIsGone(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	*underTest.tradingStrategyFailure = domains.TradingStrategyNotFound(botsTradingStrategyID)
	underTest.expectQueuedMessage(func(message string) {
		assert.Contains(t, message, "【已停擺】早盤突破 · BTCUSDT、ETHUSDT")
		assert.Contains(t, message, "原因：它用的那一份交易策略找不到了。")
	})

	storedBot := underTest.runsTheBot(t, aDueBotWatching([2]string{"BTCUSDT", ""}, [2]string{"ETHUSDT", ""}))

	assert.Equal(t, string(vo.StrategyBotStopped), storedBot.RunState)
	assert.Equal(t, string(vo.StrategyBotHaltTradingStrategyUnavailable), storedBot.HaltReason)
	for _, watchedSymbol := range storedBot.WatchedSymbols {
		assert.Empty(t, watchedSymbol.HaltReason, watchedSymbol.Symbol)
	}
	require.Len(t, *underTest.appendedRunRecords, 2)
	for _, appended := range *underTest.appendedRunRecords {
		assert.Equal(t, "hold", appended.Result)
	}
}

// expectContractSourcesOn answers each contract symbol with bars closing at its own price, which the sources read to speak for that symbol alone; a symbol whose newest candle is old is stale.
func (underTest strategyBotRunUnderTest) expectContractSourcesOn(
	closePrices map[string]string, signalsByClosePrice map[string]vo.SignalVo, staleSymbols ...string,
) {
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID, Script: scriptOfStrategyScript(id),
				ResultType: "signal", MarketDataKind: string(vo.MarketDataKindContractKCandle),
			}, nil
		}).AnyTimes()

	for symbol, closePrice := range closePrices {
		bars := storedContractCandlesAt(at(9, 10))
		bars[0].Symbol = symbol
		bars[0].Close = decimal.RequireFromString(closePrice)
		underTest.kCandleContractRepository.EXPECT().
			FindLatestBefore(gomock.Any(), symbol, gomock.Any(), gomock.Any()).Return(bars, nil).AnyTimes()

		latestOpenTime := at(9, 14)
		for _, staleSymbol := range staleSymbols {
			if staleSymbol == symbol {
				latestOpenTime = at(8, 50)
			}
		}
		latestCandle := storedContractCandlesAt(latestOpenTime)[0]
		latestCandle.Symbol = symbol
		latestCandle.Close = decimal.RequireFromString(closePrice)
		underTest.kCandleContractRepository.EXPECT().FindLatest(gomock.Any(), symbol, 1).
			Return([]entities.KCandleContract{latestCandle}, nil).AnyTimes()
	}

	underTest.contractIndicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, _ domains.IndicatorResultTypeDomain,
			bars []vo.ContractKCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			closePrice := decimal.NewFromFloat(bars[len(bars)-1].Close).String()

			return map[string]vo.IndicatorValueVo{
				vo.SignalIndicatorKey: {Signal: signalsByClosePrice[closePrice]},
			}, nil
		}).AnyTimes()
	underTest.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, symbol string) (entities.ContractTradingSymbol, bool, error) {
			specification := aContractSpecification()
			specification.Symbol = symbol

			return specification, true, nil
		}).AnyTimes()
	underTest.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	underTest.contractFundingRateSettlementRepository.EXPECT().FindLatest(gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
}

func TestAMultiSymbolContractBotQueuesOneAutoOrderPerSymbol(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongOnly)
	underTest.expectContractSourcesOn(
		map[string]string{"BTCUSDT": "85000", "ETHUSDT": "4000"},
		map[string]vo.SignalVo{"85000": vo.SignalBuy, "4000": vo.SignalBuy})
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "合約突破 · BTCUSDT 永續合約") })
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "合約突破 · ETHUSDT 永續合約") })
	bot := anAutoOrderingContractBot("")
	bot.WatchedSymbols = append(bot.WatchedSymbols,
		entities.StrategyBotWatchedSymbol{StrategyBotID: strategyBotID, Symbol: "ETHUSDT"})

	underTest.runsTheBot(t, bot)

	require.Len(t, *underTest.queuedAutoOrders, 2)
	assert.Equal(t, "BTCUSDT", (*underTest.queuedAutoOrders)[0].Symbol)
	assertFigure(t, "85000", (*underTest.queuedAutoOrders)[0].ReferencePrice)
	assert.Equal(t, "ETHUSDT", (*underTest.queuedAutoOrders)[1].Symbol)
	assertFigure(t, "4000", (*underTest.queuedAutoOrders)[1].ReferencePrice)
}

func TestAMultiSymbolContractBotSkipsOnlyTheSymbolWhoseMarketStopped(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongOnly)
	underTest.expectContractSourcesOn(
		map[string]string{"BTCUSDT": "85000", "ETHUSDT": "4000"},
		map[string]vo.SignalVo{"85000": vo.SignalBuy, "4000": vo.SignalBuy}, "ETHUSDT")
	underTest.expectQueuedMessage(func(message string) { assert.Contains(t, message, "合約突破 · BTCUSDT 永續合約") })
	bot := aDueContractBot("")
	bot.WatchedSymbols = append(bot.WatchedSymbols,
		entities.StrategyBotWatchedSymbol{StrategyBotID: strategyBotID, Symbol: "ETHUSDT"})

	storedBot := underTest.runsTheBot(t, bot)

	assert.Equal(t, string(vo.SignalBuy), lastSentSignalOf(storedBot, "BTCUSDT"))
	assert.Empty(t, lastSentSignalOf(storedBot, "ETHUSDT"))
	require.Len(t, *underTest.appendedRunRecords, 2)
	assert.Equal(t, "ETHUSDT", (*underTest.appendedRunRecords)[1].Symbol)
	assert.Equal(t, "hold", (*underTest.appendedRunRecords)[1].Result)
}

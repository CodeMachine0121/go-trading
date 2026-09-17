package application_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// botRunNow is the moment every round below happens at.
var botRunNow = at(9, 15)

type strategyBotRunUnderTest struct {
	strategyBotRunApplication  *application.StrategyBotRunApplication
	strategyBotRepository      *mocks.MockIStrategyBotRepository
	kCandleRepository          *mocks.MockIKCandleRepository
	indicatorScriptProxy       *mocks.MockIIndicatorScriptProxy
	messageDeliveryProxy       *mocks.MockIMessageDeliveryProxy
	strategyScriptRepository   *mocks.MockIStrategyScriptRepository
	telegramDeliveryRepository *mocks.MockITelegramDeliveryRepository
	roundGuard                 *application.StrategyBotRoundGuard
	// tradingStrategy is the rules every round in this file reads, held by pointer
	// so that a test can change them and have the next round see the change — which
	// is exactly what a round does against a set of rules somebody has edited.
	tradingStrategy *entities.TradingStrategy
	// tradingStrategyFailure makes that read fail instead, for the one test about a
	// bot whose rules are gone.
	tradingStrategyFailure *error
	t                      *testing.T
}

// newStrategyBotRunUnderTest wires the real services and models a round goes
// through — resolving a strategy script, running its script, reading the conditions,
// working out what a failure means — and mocks only storage, script execution and
// the carrier.
func newStrategyBotRunUnderTest(t *testing.T) strategyBotRunUnderTest {
	controller := gomock.NewController(t)

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	// 歷史是每一輪都會寫的，而它寫不寫得成不是這幾個測試在問的事。
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(controller)
	strategyBotRunRecordRepository.EXPECT().
		Append(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(controller)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)

	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(botRunNow).AnyTimes()

	// 測試拿得到那把鎖，才問得出「正在跑的時候按下去會怎樣」。
	roundGuard := application.NewStrategyBotRoundGuard()

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()

	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(controller)

	tradingStrategy := aDueTradingStrategy()
	tradingStrategyFailure := error(nil)
	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), botsTradingStrategyID).
		DoAndReturn(func(_ context.Context, _ uint) (entities.TradingStrategy, error) {
			if tradingStrategyFailure != nil {
				return entities.TradingStrategy{}, tradingStrategyFailure
			}

			return tradingStrategy, nil
		}).AnyTimes()

	secretSealProxy := mocks.NewMockISecretSealProxy(controller)
	secretSealProxy.EXPECT().Unseal("sealed").Return("the-token", nil).AnyTimes()

	return strategyBotRunUnderTest{
		strategyBotRunApplication: application.NewStrategyBotRunApplication(
			service.NewStrategyBotService(
				strategyBotRepository, strategyBotRunRecordRepository, clockProxy),
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				queryMaxResults),
			service.NewTelegramDeliveryService(
				telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy),
			service.NewKCandleService(
				kCandleRepository, tradingSymbolRepository, clockProxy,
				domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}}),
				queryMaxResults),
			clockProxy,
			roundGuard,
			4,
			time.Minute,
		),
		strategyBotRepository:      strategyBotRepository,
		kCandleRepository:          kCandleRepository,
		indicatorScriptProxy:       indicatorScriptProxy,
		messageDeliveryProxy:       messageDeliveryProxy,
		strategyScriptRepository:   strategyScriptRepository,
		telegramDeliveryRepository: telegramDeliveryRepository,
		roundGuard:                 roundGuard,
		tradingStrategy:            &tradingStrategy,
		tradingStrategyFailure:     &tradingStrategyFailure,
		t:                          t,
	}
}

// expectDeliverySetting 讓這位使用者有一個講得出話的地方。
//
// 它不是預設就有的：有沒有設定是這一段少數幾個**改得動結果**的前提之一——
// 設定被移除時機器人要停擺，而不是靜靜跳過。
// expectNoRoundMessage 說的是「這一輪不對市場說話」。
//
// 它不是「一則都不送」：機器人**關於它自己**的動靜——被系統停下來了——是另一回事，
// 而那一則正是使用者最需要收到的。兩者用開頭那個括號分得出來。
func (underTest strategyBotRunUnderTest) expectNoRoundMessage() {
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.NotContains(underTest.t, message, "參考價",
				"這一輪不該對市場說話")

			return vo.DeliveryFailureNone, nil
		}).AnyTimes()
}

func (underTest strategyBotRunUnderTest) expectDeliverySetting() {
	underTest.telegramDeliveryRepository.EXPECT().FindOneByUser(gomock.Any(), gomock.Any()).
		Return(entities.TelegramDelivery{
			UserID: strategyBotOwnerID, SealedBotToken: "sealed", ChatID: "987654",
		}, nil).AnyTimes()
}

// aDueBot is one running bot, due now, following the rules aDueTradingStrategy
// describes.
func aDueBot(lastSentSignal string) entities.StrategyBot {
	return entities.StrategyBot{
		ID: strategyBotID, OwnerID: strategyBotOwnerID, Name: "早盤突破", Symbol: "BTCUSDT",
		TradingStrategyID:      botsTradingStrategyID,
		TradingStrategy:        entities.TradingStrategy{ID: botsTradingStrategyID, Name: "黃金交叉"},
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		// 這一輪是憑這個時刻被領走的。送出前與寫回前都會再確認它沒有被別人動過。
		NextRunAt:      botRunNow.Add(-time.Minute),
		LastSentSignal: lastSentSignal,
	}
}

// aDueTradingStrategy is two sources and the conditions "buy when both say buy" and
// "sell when A says sell".
func aDueTradingStrategy() entities.TradingStrategy {
	return entities.TradingStrategy{
		ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, Name: "黃金交叉",
		SignalSources: []entities.TradingStrategySignalSource{
			{ID: 20, TradingStrategyID: botsTradingStrategyID, Label: "A",
				StrategyScriptID: 9, AggregationInterval: "1h"},
			{ID: 21, TradingStrategyID: botsTradingStrategyID, Label: "B",
				StrategyScriptID: 10, AggregationInterval: "5m"},
		},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 10, TradingStrategyID: botsTradingStrategyID, Side: "buy",
				Operator: string(vo.ConditionOperatorAnd)},
			{ID: 11, TradingStrategyID: botsTradingStrategyID, Side: "buy", ParentID: parentOf(10),
				Position: 0, SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
			{ID: 12, TradingStrategyID: botsTradingStrategyID, Side: "buy", ParentID: parentOf(10),
				Position: 1, SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
			{ID: 13, TradingStrategyID: botsTradingStrategyID, Side: "sell",
				SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
		},
	}
}

func parentOf(id uint) *uint {
	return &id
}

// expectSources makes both strategy scripts resolvable and has each one's script say its
// own signal.
//
// Which signal belongs to which source is keyed on the script rather than on the
// order the runner is called in, because the sources run side by side: an assertion
// that depended on that order would pass or fail depending on which goroutine won.
func (underTest strategyBotRunUnderTest) expectSources(
	firstSourceSignal vo.SignalVo, secondSourceSignal vo.SignalVo,
) {
	signalsByScript := map[string]vo.SignalVo{
		scriptOfStrategyScript(9):  firstSourceSignal,
		scriptOfStrategyScript(10): secondSourceSignal,
	}

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategyScript(id), ResultType: "signal",
			}, nil
		}).AnyTimes()

	underTest.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "100")}, nil).AnyTimes()

	underTest.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, script string, _ domains.IndicatorResultTypeDomain,
			_ []vo.KCandleVo, _ domains.StrategyScriptParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			return map[string]vo.IndicatorValueVo{
				vo.SignalIndicatorKey: {Signal: signalsByScript[script]},
			}, nil
		}).AnyTimes()
}

// scriptOfStrategyScript gives each strategy script a script of its own, which is what lets a
// test say which source said what without depending on when each one ran.
func scriptOfStrategyScript(id uint) string {
	return fmt.Sprintf("the script of %d", id)
}

func TestStrategyBotRunApplicationSendsAConclusionThatChanged(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil)

	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, credential vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Equal(t, "the-token", credential.BotToken)
			assert.Contains(t, message, "【買入】早盤突破 · BTCUSDT")
			assert.Contains(t, message, "64180.5")

			return vo.DeliveryFailureNone, nil
		})

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.SignalBuy), bot.LastSentSignal)
			assert.False(t, bot.Conflicting)
			assert.Equal(t, botRunNow.Add(5*time.Minute), bot.NextRunAt)

			return nil
		})

	roundsRun, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	assert.Equal(t, 1, roundsRun)
}

func TestStrategyBotRunApplicationSaysNothingWhenTheConclusionHasNotChanged(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot(string(vo.SignalBuy))}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(string(vo.SignalBuy)), nil)
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	// A bot waking every five minutes on a condition that holds for an hour reaches
	// the same conclusion twelve times. Sending all twelve gets the bot muted.
	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationMarksAConflictAndSaysNothing(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	// A says sell — which is the whole sell condition — while the buy condition is
	// "A and B both buy". To hold both at once the bot needs a buy condition A also
	// satisfies, so this one uses a bot whose conditions overlap on purpose.
	conflictingBot := aDueBot(string(vo.SignalBuy))
	underTest.tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 10, TradingStrategyID: botsTradingStrategyID, Side: "buy",
			SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, TradingStrategyID: botsTradingStrategyID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
	}

	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalSell, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{conflictingBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(conflictingBot, nil)
	underTest.expectNoRoundMessage()

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.True(t, bot.Conflicting)
			// No side is picked, and what it last said stands.
			assert.Equal(t, string(vo.SignalBuy), bot.LastSentSignal)
			assert.Equal(t, string(vo.StrategyBotRunning), bot.RunState)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationHaltsForFailuresThatWillNeverFixThemselves(t *testing.T) {
	testCases := []struct {
		name                    string
		strategyScriptFindError error
		expectedHaltReason      vo.StrategyBotHaltReasonVo
	}{
		{
			name:                    "a strategy script that can no longer be seen",
			strategyScriptFindError: domains.StrategyScriptNotFound(9),
			expectedHaltReason:      vo.StrategyBotHaltStrategyScriptUnavailable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			underTest.expectDeliverySetting()

			underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
				Return(entities.StrategyScript{}, testCase.strategyScriptFindError).AnyTimes()
			underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
				Return([]entities.StrategyBot{aDueBot("")}, nil)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
				Return(aDueBot(""), nil).AnyTimes()
			underTest.expectNoRoundMessage()

			underTest.strategyBotRepository.EXPECT().
				UpdateRunState(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
					assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
					assert.Equal(t, string(testCase.expectedHaltReason), bot.HaltReason)

					return nil
				})

			_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

			require.NoError(t, runError)
		})
	}
}

func TestStrategyBotRunApplicationHaltsWhenAScriptWillNotRun(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategyScript(id), ResultType: "signal",
			}, nil
		}).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "100")}, nil).AnyTimes()
	underTest.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, domains.ErrIndicatorScriptFailed).AnyTimes()
	underTest.expectNoRoundMessage()

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
			assert.Equal(t, string(vo.StrategyBotHaltScriptFailed), bot.HaltReason)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationKeepsRunningWhenTheCandlesAreNotThereYet(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategyScript(id), ResultType: "signal",
			}, nil
		}).AnyTimes()
	// Nothing stored yet: the calculation refuses for want of candles, which is a
	// failure that tomorrow fixes by itself.
	underTest.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{}, nil).AnyTimes()

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot(string(vo.SignalBuy))}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(string(vo.SignalBuy)), nil)
	underTest.expectNoRoundMessage()

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			// Halting here would switch off every bot in the system each weekend.
			assert.Equal(t, string(vo.StrategyBotRunning), bot.RunState)
			assert.Empty(t, bot.HaltReason)
			assert.Equal(t, string(vo.SignalBuy), bot.LastSentSignal)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationReadsTelegramsRefusalTheWayItWasMeant(t *testing.T) {
	testCases := []struct {
		name               string
		failureReason      vo.DeliveryFailureReasonVo
		expectedRunState   vo.StrategyBotRunStateVo
		expectedHaltReason vo.StrategyBotHaltReasonVo
	}{
		{
			name:               "a rejected token stops the bot",
			failureReason:      vo.DeliveryFailureCredentialRejected,
			expectedRunState:   vo.StrategyBotStopped,
			expectedHaltReason: vo.StrategyBotHaltCredentialRejected,
		},
		{
			name:               "an unknown chat stops the bot",
			failureReason:      vo.DeliveryFailureDestinationNotFound,
			expectedRunState:   vo.StrategyBotStopped,
			expectedHaltReason: vo.StrategyBotHaltDestinationNotFound,
		},
		{
			name:             "an unreachable Telegram waits for the next round",
			failureReason:    vo.DeliveryFailureUnreachable,
			expectedRunState: vo.StrategyBotRunning,
		},
		{
			name:             "a Telegram that answered too late waits for the next round",
			failureReason:    vo.DeliveryFailureTimedOut,
			expectedRunState: vo.StrategyBotRunning,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			underTest.expectDeliverySetting()
			underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

			underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
				Return([]entities.StrategyBot{aDueBot("")}, nil)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
				Return(aDueBot(""), nil).AnyTimes()
			underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
				Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil)
			// 這一輪的那一則回它指定的失敗；如果因此停擺，機器人自己的那一則照樣送得出去。
			underTest.messageDeliveryProxy.EXPECT().
				Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
				DoAndReturn(func(
					_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
				) (vo.DeliveryFailureReasonVo, error) {
					if strings.Contains(message, "已停擺") {
						return vo.DeliveryFailureNone, nil
					}

					return testCase.failureReason, nil
				}).AnyTimes()

			underTest.strategyBotRepository.EXPECT().
				UpdateRunState(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
					assert.Equal(t, string(testCase.expectedRunState), bot.RunState)
					assert.Equal(t, string(testCase.expectedHaltReason), bot.HaltReason)
					// Nothing arrived, so nothing was said: the next round offers
					// the same conclusion again rather than assuming it got through.
					assert.Empty(t, bot.LastSentSignal)

					return nil
				})

			_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

			require.NoError(t, runError)
		})
	}
}

func TestStrategyBotRunApplicationStillSendsWhenThereIsNoPriceToQuote(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return(nil, errors.New("the database went away"))

	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			// The conclusion is the part somebody acts on; the price is context.
			assert.Contains(t, message, "【買入】")
			assert.Contains(t, message, "讀不到這個交易標的的最新 K 線")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationRunsNothingWhenNothingIsDue(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{}, nil)

	roundsRun, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	assert.Equal(t, 0, roundsRun)
}

func TestStrategyBotRunApplicationReportsAFailedScan(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return(nil, errors.New("the database went away"))

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	assert.Error(t, runError)
}

func TestStrategyBotRoundGuardKeepsOneBotToOneRoundAtATime(t *testing.T) {
	roundGuard := application.NewStrategyBotRoundGuard()

	assert.True(t, roundGuard.TryEnter(strategyBotID))
	// A round waiting behind the one in flight would read the same candles and
	// arrive as a second message saying the same thing.
	assert.False(t, roundGuard.TryEnter(strategyBotID))
	// A different bot is nothing to do with it.
	assert.True(t, roundGuard.TryEnter(strategyBotID+1))

	roundGuard.Leave(strategyBotID)
	assert.True(t, roundGuard.TryEnter(strategyBotID))
}

func TestStrategyBotRunApplicationSkipsARoundWhoseStoredConditionNoLongerReads(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// A condition naming a label this bot no longer declares. It is not one of the
	// four things a person can go and correct, so it skips rather than halts — the
	// rule that covers anything unrecognised.
	brokenBot := aDueBot("")
	underTest.tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 10, TradingStrategyID: botsTradingStrategyID, Side: "buy",
			SourceLabel: "Z", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, TradingStrategyID: botsTradingStrategyID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
	}

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{brokenBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(brokenBot, nil)
	underTest.expectNoRoundMessage()
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

func TestStrategyBotRunApplicationSkipsARoundWhoseMessageCouldNotBeBuilt(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil)
	// Not Telegram refusing — this side failing to ask at all. It is not one of the
	// four reasons, so the round waits rather than stopping the bot.
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.DeliveryFailureNone, errors.New("the request could not be built"))

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotRunning), bot.RunState)
			assert.Empty(t, bot.LastSentSignal)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationLeavesABotThatIsAlreadyMidRound(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot(""), aDueBot("")}, nil)
	// Both entries name the same bot, so the second finds the claim taken and is
	// skipped. Anything else would send the same message twice.
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).Times(1)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalHold, vo.SignalHold)
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Return(nil).Times(1)

	roundsRun, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
	assert.Equal(t, 1, roundsRun)
}

func TestStrategyBotRunApplicationCarriesOnWhenARoundCannotBeBookedIn(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest strategyBotRunUnderTest)
	}{
		{
			name: "the bot cannot be read back to record the round",
			arrange: func(underTest strategyBotRunUnderTest) {
				underTest.expectDeliverySetting()
				underTest.expectSources(vo.SignalHold, vo.SignalHold)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
		},
		{
			name: "the round cannot be written back",
			arrange: func(underTest strategyBotRunUnderTest) {
				underTest.expectDeliverySetting()
				underTest.expectSources(vo.SignalHold, vo.SignalHold)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(aDueBot(""), nil).AnyTimes()
				underTest.strategyBotRepository.EXPECT().
					UpdateRunState(gomock.Any(), gomock.Any()).
					Return(errors.New("the database went away"))
			},
		},
		{
			name: "a halt cannot be written back",
			arrange: func(underTest strategyBotRunUnderTest) {
				underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
					Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(9)).AnyTimes()
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(aDueBot(""), nil).AnyTimes()
				underTest.strategyBotRepository.EXPECT().
					UpdateRunState(gomock.Any(), gomock.Any()).
					Return(errors.New("the database went away"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
				Return([]entities.StrategyBot{aDueBot("")}, nil)
			testCase.arrange(underTest)

			// One bot's round failing to be recorded is that bot's problem. The scan
			// still reports the round as run, and the bots beside it are untouched:
			// a scan that gave up here would stop every other bot in the system.
			roundsRun, runError := underTest.strategyBotRunApplication.RunDueRounds(
				context.Background())

			require.NoError(t, runError)
			assert.Equal(t, 1, roundsRun)
		})
	}
}

func TestStrategyBotRunApplicationSkipsARoundWhoseStoredSellConditionNoLongerReads(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// The sell tree is checked as thoroughly as the buy one. Checked only on one
	// side, a bot with a broken sell condition would keep concluding "buy" from a
	// tree nobody had looked at.
	brokenBot := aDueBot("")
	underTest.tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 10, TradingStrategyID: botsTradingStrategyID, Side: "buy",
			SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, TradingStrategyID: botsTradingStrategyID, Side: "sell",
			SourceLabel: "Z", ExpectedSignal: string(vo.SignalSell)},
	}

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{brokenBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(brokenBot, nil)
	underTest.expectNoRoundMessage()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationStillSendsWhenNoCandleIsStoredAtAll(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{}, nil)

	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			// Nothing stored is an ordinary state, not a failure — and the message
			// still goes out, because the conclusion is the part somebody acts on.
			assert.Contains(t, message, "【買入】")
			assert.Contains(t, message, "讀不到這個交易標的的最新 K 線")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationReadsEachSourceAtItsOwnCoarseness(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategyScript(id), ResultType: "signal",
			}, nil
		}).AnyTimes()
	underTest.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{
			vo.SignalIndicatorKey: {Signal: vo.SignalHold},
		}, nil).AnyTimes()

	// Each source stops at the start of the interval it is still inside: the hourly
	// one at the top of this hour, the five-minute one at 09:15. Two sources sharing
	// one coarseness could not tell an hourly average for direction from a
	// five-minute oscillator for timing, which is the whole reason coarseness lives
	// on the source and not on the bot.
	cutoffs := make(chan time.Time, 8)
	underTest.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ string, cutoffTime time.Time, _ int,
		) ([]entities.KCandle, error) {
			cutoffs <- cutoffTime

			return []entities.KCandle{kCandleAt(at(9, 10), "100")}, nil
		}).AnyTimes()

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())
	require.NoError(t, runError)

	close(cutoffs)
	readCutoffs := map[time.Time]bool{}
	for cutoff := range cutoffs {
		readCutoffs[cutoff.UTC()] = true
	}

	assert.True(t, readCutoffs[at(9, 0)], "the hourly source should stop at the top of the hour")
	assert.True(t, readCutoffs[at(9, 15)], "the five-minute source should stop at 09:15")
}

func TestStrategyBotRunApplicationSaysNothingForABotDeletedMidRound(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	// 這一輪算完之後它就不在了。
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(entities.StrategyBot{}, domains.StrategyBotNotFound(strategyBotID)).AnyTimes()

	// 一則來自剛被刪掉的機器人的訊息是一輪絕對不能送的東西：
	// 它的擁有者已經沒有任何地方可以回去看它是從哪來的。
	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationDoesNotUndoARestartThatHappenedMidRound(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// 這一輪開始之後，擁有者停了又啟動——啟動把 NextRunAt 設成現在、把上次訊號清空。
	restarted := aDueBot("")
	restarted.NextRunAt = botRunNow

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(restarted, nil).AnyTimes()

	// 這一輪講的已經不是這台機器人現在的狀態了，所以它一個字都不寫回去——
	// 寫回去的話，剛按下的那個「立刻跑第一輪」會被推遲一個間隔，
	// 而被清空的上次訊號會復活，把啟動後的第一個結論吃掉。
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Times(0)
	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationHaltsWhenTheDeliverySettingIsGone(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), gomock.Any()).
		Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured).AnyTimes()

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil).AnyTimes()

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			// 沒地方講話的機器人，執行中與已停止之間沒有任何差別——
			// 而這正是當初拒絕啟動它的理由。跳過這一輪的話，它會永遠靜靜地
			// 保持「執行中」、沒有原因、什麼都不說。
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
			assert.Equal(t, string(vo.StrategyBotHaltDeliveryNotConfigured), bot.HaltReason)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationRunsARoundByHandDownTheSamePath(t *testing.T) {
	// 按下去看到的，必須就是它自己跑會做的事——不然這顆鍵沒辦法用來確認任何事。
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil).AnyTimes()

	delivered := ""
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			delivered = message

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, runError)
	assert.Contains(t, delivered, "【買入】")
}

func TestStrategyBotRunApplicationRunsAStoppedBotByHand(t *testing.T) {
	// 派它出去之前先試一次，是確認它說的是不是你要的意思最普通的做法。
	// 非要執行中才試得動的話，唯一的測法就是讓它一直跑著。
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	stoppedBot := aDueBot("")
	stoppedBot.RunState = string(vo.StrategyBotStopped)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(stoppedBot, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil).AnyTimes()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.DeliveryFailureNone, nil)

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, updated entities.StrategyBot) error {
			// 跑一輪不會把它打開——那是電源鍵的事。
			assert.Equal(t, string(vo.StrategyBotStopped), updated.RunState)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationRefusesAHandPressedRoundWhileOneIsInFlight(t *testing.T) {
	// 排隊的那一輪會讀到同樣的 K 線、得到同樣的答案，而後跑完的那一個會發現
	// 這台機器人已經往前走了。
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil).AnyTimes()
	require.True(t, underTest.roundGuard.TryEnter(strategyBotID))

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.ErrorIs(t, runError, domains.ErrStrategyBotAlreadyRunningARound)
}

func TestStrategyBotRunApplicationRefusesToRunSomebodyElsesBotByHand(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID+99, strategyBotID)

	require.ErrorIs(t, runError, domains.ErrStrategyBotNotFound)
}

// A bot whose rules are gone has nothing left to run, and waiting fixes nothing.
//
// Deleting a set of rules is refused while any bot follows it, so this is very nearly
// unreachable — but "very nearly" is why it halts rather than skips: a bot reporting
// itself as running while saying nothing for ever is the one outcome its owner can
// neither see nor fix.
func TestStrategyBotRunApplicationHaltsABotWhoseRulesAreGone(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
	*underTest.tradingStrategyFailure = domains.TradingStrategyNotFound(botsTradingStrategyID)
	// Halting says so out loud, and the message names which of the six reasons it
	// was — a bot that stops without saying why leaves its owner nothing to act on.
	underTest.expectDeliverySetting()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "那一份交易策略找不到了")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
			assert.Equal(t, string(vo.StrategyBotHaltTradingStrategyUnavailable), bot.HaltReason)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

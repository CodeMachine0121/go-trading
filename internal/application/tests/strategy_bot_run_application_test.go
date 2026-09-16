package application_test

import (
	"context"
	"errors"
	"fmt"
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
	strategyBotRunApplication *application.StrategyBotRunApplication
	strategyBotRepository     *mocks.MockIStrategyBotRepository
	kCandleRepository         *mocks.MockIKCandleRepository
	indicatorScriptProxy      *mocks.MockIIndicatorScriptProxy
	messageDeliveryProxy      *mocks.MockIMessageDeliveryProxy
	strategyRepository        *mocks.MockIStrategyRepository
}

// newStrategyBotRunUnderTest wires the real services and models a round goes
// through — resolving a strategy, running its script, reading the conditions,
// working out what a failure means — and mocks only storage, script execution and
// the carrier.
func newStrategyBotRunUnderTest(t *testing.T) strategyBotRunUnderTest {
	controller := gomock.NewController(t)

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(controller)
	strategyRepository := mocks.NewMockIStrategyRepository(controller)

	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(botRunNow).AnyTimes()

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()

	publishedStrategyRepository := mocks.NewMockIPublishedStrategyRepository(controller)
	publishedStrategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategy{}, domains.ErrStrategyNotPublished).AnyTimes()

	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(controller)
	telegramDeliveryRepository.EXPECT().FindOneByUser(gomock.Any(), gomock.Any()).
		Return(entities.TelegramDelivery{
			UserID: strategyBotOwnerID, SealedBotToken: "sealed", ChatID: "987654",
		}, nil).AnyTimes()

	secretSealProxy := mocks.NewMockISecretSealProxy(controller)
	secretSealProxy.EXPECT().Unseal("sealed").Return("the-token", nil).AnyTimes()

	return strategyBotRunUnderTest{
		strategyBotRunApplication: application.NewStrategyBotRunApplication(
			service.NewStrategyBotService(strategyBotRepository, clockProxy),
			service.NewStrategyService(strategyRepository, publishedStrategyRepository),
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
			application.NewStrategyBotRoundGuard(),
			4,
			time.Minute,
		),
		strategyBotRepository: strategyBotRepository,
		kCandleRepository:     kCandleRepository,
		indicatorScriptProxy:  indicatorScriptProxy,
		messageDeliveryProxy:  messageDeliveryProxy,
		strategyRepository:    strategyRepository,
	}
}

// aDueBot is one running bot with two sources and the conditions
// "buy when both say buy" and "sell when A says sell".
func aDueBot(lastSentSignal string) entities.StrategyBot {
	return entities.StrategyBot{
		ID: strategyBotID, OwnerID: strategyBotOwnerID, Name: "早盤突破", Symbol: "BTCUSDT",
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		LastSentSignal:         lastSentSignal,
		SignalSources: []entities.StrategyBotSignalSource{
			{ID: 20, StrategyBotID: strategyBotID, Label: "A", StrategyID: 9, AggregationInterval: "5m"},
			{ID: 21, StrategyBotID: strategyBotID, Label: "B", StrategyID: 10, AggregationInterval: "5m"},
		},
		ConditionNodes: []entities.StrategyBotConditionNode{
			{ID: 10, StrategyBotID: strategyBotID, Side: "buy",
				Operator: string(vo.ConditionOperatorAnd)},
			{ID: 11, StrategyBotID: strategyBotID, Side: "buy", ParentID: parentOf(10),
				Position: 0, SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
			{ID: 12, StrategyBotID: strategyBotID, Side: "buy", ParentID: parentOf(10),
				Position: 1, SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
			{ID: 13, StrategyBotID: strategyBotID, Side: "sell",
				SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
		},
	}
}

func parentOf(id uint) *uint {
	return &id
}

// expectSources makes both strategies resolvable and has each one's script say its
// own signal.
//
// Which signal belongs to which source is keyed on the script rather than on the
// order the runner is called in, because the sources run side by side: an assertion
// that depended on that order would pass or fail depending on which goroutine won.
func (underTest strategyBotRunUnderTest) expectSources(
	firstSourceSignal vo.SignalVo, secondSourceSignal vo.SignalVo,
) {
	signalsByScript := map[string]vo.SignalVo{
		scriptOfStrategy(9):  firstSourceSignal,
		scriptOfStrategy(10): secondSourceSignal,
	}

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.Strategy, error) {
			return entities.Strategy{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategy(id), ResultType: "signal",
			}, nil
		}).AnyTimes()

	underTest.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "100")}, nil).AnyTimes()

	underTest.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, script string, _ domains.IndicatorResultTypeDomain,
			_ []vo.KCandleVo, _ domains.StrategyParametersDomain,
		) (map[string]vo.IndicatorValueVo, error) {
			return map[string]vo.IndicatorValueVo{
				vo.SignalIndicatorKey: {Signal: signalsByScript[script]},
			}, nil
		}).AnyTimes()
}

// scriptOfStrategy gives each strategy a script of its own, which is what lets a
// test say which source said what without depending on when each one ran.
func scriptOfStrategy(id uint) string {
	return fmt.Sprintf("the script of %d", id)
}

func TestStrategyBotRunApplicationSendsAConclusionThatChanged(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
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
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot(string(vo.SignalBuy))}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(string(vo.SignalBuy)), nil)
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	// A bot waking every five minutes on a condition that holds for an hour reaches
	// the same conclusion twelve times. Sending all twelve gets the bot muted.
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationMarksAConflictAndSaysNothing(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	// A says sell — which is the whole sell condition — while the buy condition is
	// "A and B both buy". To hold both at once the bot needs a buy condition A also
	// satisfies, so this one uses a bot whose conditions overlap on purpose.
	conflictingBot := aDueBot(string(vo.SignalBuy))
	conflictingBot.ConditionNodes = []entities.StrategyBotConditionNode{
		{ID: 10, StrategyBotID: strategyBotID, Side: "buy",
			SourceLabel: "B", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, StrategyBotID: strategyBotID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
	}

	underTest.expectSources(vo.SignalSell, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{conflictingBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(conflictingBot, nil)
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
		name               string
		strategyFindError  error
		expectedHaltReason vo.StrategyBotHaltReasonVo
	}{
		{
			name:               "a strategy that can no longer be seen",
			strategyFindError:  domains.StrategyNotFound(9),
			expectedHaltReason: vo.StrategyBotHaltStrategyUnavailable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)

			underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
				Return(entities.Strategy{}, testCase.strategyFindError).AnyTimes()
			underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
				Return([]entities.StrategyBot{aDueBot("")}, nil)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
				Return(aDueBot(""), nil)
			underTest.messageDeliveryProxy.EXPECT().
				Deliver(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.Strategy, error) {
			return entities.Strategy{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategy(id), ResultType: "signal",
			}, nil
		}).AnyTimes()
	underTest.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "100")}, nil).AnyTimes()
	underTest.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, domains.ErrIndicatorScriptFailed).AnyTimes()

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
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

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.Strategy, error) {
			return entities.Strategy{
				ID: id, OwnerID: strategyBotOwnerID,
				Script: scriptOfStrategy(id), ResultType: "signal",
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
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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
			underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

			underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
				Return([]entities.StrategyBot{aDueBot("")}, nil)
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
				Return(aDueBot(""), nil)
			underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
				Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil)
			underTest.messageDeliveryProxy.EXPECT().
				Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(testCase.failureReason, nil)

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
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
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
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// A condition naming a label this bot no longer declares. It is not one of the
	// four things a person can go and correct, so it skips rather than halts — the
	// rule that covers anything unrecognised.
	brokenBot := aDueBot("")
	brokenBot.ConditionNodes = []entities.StrategyBotConditionNode{
		{ID: 10, StrategyBotID: strategyBotID, Side: "buy",
			SourceLabel: "Z", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, StrategyBotID: strategyBotID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
	}

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{brokenBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(brokenBot, nil)
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
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
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
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
				underTest.expectSources(vo.SignalHold, vo.SignalHold)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
		},
		{
			name: "the round cannot be written back",
			arrange: func(underTest strategyBotRunUnderTest) {
				underTest.expectSources(vo.SignalHold, vo.SignalHold)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(aDueBot(""), nil)
				underTest.strategyBotRepository.EXPECT().
					UpdateRunState(gomock.Any(), gomock.Any()).
					Return(errors.New("the database went away"))
			},
		},
		{
			name: "a halt cannot be written back",
			arrange: func(underTest strategyBotRunUnderTest) {
				underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
					Return(entities.Strategy{}, domains.StrategyNotFound(9)).AnyTimes()
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(aDueBot(""), nil)
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
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// The sell tree is checked as thoroughly as the buy one. Checked only on one
	// side, a bot with a broken sell condition would keep concluding "buy" from a
	// tree nobody had looked at.
	brokenBot := aDueBot("")
	brokenBot.ConditionNodes = []entities.StrategyBotConditionNode{
		{ID: 10, StrategyBotID: strategyBotID, Side: "buy",
			SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
		{ID: 13, StrategyBotID: strategyBotID, Side: "sell",
			SourceLabel: "Z", ExpectedSignal: string(vo.SignalSell)},
	}

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{brokenBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(brokenBot, nil)
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationStillSendsWhenNoCandleIsStoredAtAll(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
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

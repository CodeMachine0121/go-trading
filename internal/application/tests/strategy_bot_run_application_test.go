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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var botRunNow = at(9, 15)

type strategyBotRunUnderTest struct {
	strategyBotRunApplication               *application.StrategyBotRunApplication
	strategyBotRepository                   *mocks.MockIStrategyBotRepository
	kCandleRepository                       *mocks.MockIKCandleRepository
	indicatorScriptProxy                    *mocks.MockIIndicatorScriptProxy
	messageDeliveryProxy                    *mocks.MockIMessageDeliveryProxy
	strategyScriptRepository                *mocks.MockIStrategyScriptRepository
	telegramDeliveryRepository              *mocks.MockITelegramDeliveryRepository
	kCandleContractRepository               *mocks.MockIKCandleContractRepository
	contractIndicatorScriptProxy            *mocks.MockIContractIndicatorScriptProxy
	contractTradingSymbolRepository         *mocks.MockIContractTradingSymbolRepository
	contractMaintenanceMarginTierRepository *mocks.MockIContractMaintenanceMarginTierRepository
	contractFundingRateSettlementRepository *mocks.MockIContractFundingRateSettlementRepository
	roundGuard                              *application.StrategyBotRoundGuard
	// Held by pointer so a test can edit the rules and the next round sees the change.
	tradingStrategy        *entities.TradingStrategy
	tradingStrategyFailure *error
	// Captured by the fixture's own AnyTimes expectation, since gomock would match it before any later one a test adds.
	appendedRunRecords *[]dto.StrategyBotRunRecordWriteDto
	// publishedStrategyScriptIDs are on the marketplace; everything else reads as unpublished.
	publishedStrategyScriptIDs map[uint]bool
	t                          *testing.T
}

// newStrategyBotRunUnderTest wires the real services a round goes through, mocking only storage, script execution and the carrier.
func newStrategyBotRunUnderTest(t *testing.T) strategyBotRunUnderTest {
	controller := gomock.NewController(t)

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	// 歷史寫入與這些測試無關，一律放行。
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(controller)
	appendedRunRecords := []dto.StrategyBotRunRecordWriteDto{}
	strategyBotRunRecordRepository.EXPECT().
		Append(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, writeDto dto.StrategyBotRunRecordWriteDto) error {
			appendedRunRecords = append(appendedRunRecords, writeDto)

			return nil
		}).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(controller)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)

	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(botRunNow).AnyTimes()

	// 讓測試能自己佔住鎖，模擬「正在跑的時候按下去」。
	roundGuard := application.NewStrategyBotRoundGuard()

	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(controller)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).
		Return(entities.TradingSymbol{Market: string(vo.MarketCrypto)}, true, nil).AnyTimes()

	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptIDs := map[uint]bool{}
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.PublishedStrategyScript, error) {
			if publishedStrategyScriptIDs[id] {
				return entities.PublishedStrategyScript{StrategyScriptID: id}, nil
			}

			return entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished
		}).AnyTimes()

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

	// 只有合約機器人會讀到；資金費率與持倉統計與這些測試無關。
	kCandleContractRepository := mocks.NewMockIKCandleContractRepository(controller)
	contractFundingRateSettlementRepository := mocks.NewMockIContractFundingRateSettlementRepository(controller)
	contractFundingRateSettlementRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	contractFundingRateSettlementRepository.EXPECT().FindLatestBefore(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(entities.ContractFundingRateSettlement{}, false, nil).AnyTimes()
	contractPositionStatisticRepository := mocks.NewMockIContractPositionStatisticRepository(controller)
	contractPositionStatisticRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	contractIndicatorScriptProxy := mocks.NewMockIContractIndicatorScriptProxy(controller)
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(controller)
	contractMaintenanceMarginTierRepository := mocks.NewMockIContractMaintenanceMarginTierRepository(controller)
	marketCatalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})

	return strategyBotRunUnderTest{
		strategyBotRunApplication: application.NewStrategyBotRunApplication(
			service.NewStrategyBotService(
				strategyBotRepository, strategyBotRunRecordRepository,
				contractTradingSymbolRepository, contractMaintenanceMarginTierRepository,
				contractFundingRateSettlementRepository, clockProxy),
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewIndicatorCalculationService(
				kCandleRepository, tradingSymbolRepository, indicatorScriptProxy, clockProxy,
				marketCatalog, queryMaxResults),
			service.NewContractIndicatorCalculationService(
				kCandleContractRepository, contractFundingRateSettlementRepository,
				contractPositionStatisticRepository, contractIndicatorScriptProxy, clockProxy,
				marketCatalog, queryMaxResults),
			service.NewTelegramDeliveryService(
				telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy),
			service.NewKCandleService(
				kCandleRepository, tradingSymbolRepository, clockProxy, marketCatalog, queryMaxResults),
			service.NewKCandleContractService(
				kCandleContractRepository, clockProxy, marketCatalog, queryMaxResults),
			clockProxy,
			roundGuard,
			4,
			time.Minute,
		),
		strategyBotRepository:        strategyBotRepository,
		kCandleRepository:            kCandleRepository,
		indicatorScriptProxy:         indicatorScriptProxy,
		messageDeliveryProxy:         messageDeliveryProxy,
		strategyScriptRepository:     strategyScriptRepository,
		telegramDeliveryRepository:   telegramDeliveryRepository,
		kCandleContractRepository:    kCandleContractRepository,
		contractIndicatorScriptProxy: contractIndicatorScriptProxy,

		contractTradingSymbolRepository:         contractTradingSymbolRepository,
		contractMaintenanceMarginTierRepository: contractMaintenanceMarginTierRepository,
		contractFundingRateSettlementRepository: contractFundingRateSettlementRepository,
		roundGuard:                              roundGuard,
		tradingStrategy:                         &tradingStrategy,
		tradingStrategyFailure:                  &tradingStrategyFailure,
		appendedRunRecords:                      &appendedRunRecords,
		t:                                       t,
		publishedStrategyScriptIDs:              publishedStrategyScriptIDs,
	}
}

// expectNoRoundMessage 只禁止對市場說話的訊息；機器人關於自己停擺的通知仍放行。
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

func aDueBot(lastSentSignal string) entities.StrategyBot {
	return entities.StrategyBot{
		ID: strategyBotID, OwnerID: strategyBotOwnerID, Name: "早盤突破", Symbol: "BTCUSDT",
		TradingStrategyID:      botsTradingStrategyID,
		TradingStrategy:        entities.TradingStrategy{ID: botsTradingStrategyID, Name: "黃金交叉"},
		TriggerIntervalMinutes: 5,
		RunState:               string(vo.StrategyBotRunning),
		// 送出前與寫回前都會以此時刻確認這台機器人沒被別人動過。
		NextRunAt:      botRunNow.Add(-time.Minute),
		LastSentSignal: lastSentSignal,
	}
}

// aDueTradingStrategy buys when A and B both say buy and sells when A says sell, on a spot
// account so these tests pin the exact wording of a spot bot's message.
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

// expectSources keys each signal on the script, not call order, because sources run concurrently.
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

	// A repeated conclusion is not resent, or a long-held condition would spam the owner.
	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationMarksAConflictAndSaysNothing(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	// Conditions overlap on purpose so buy (B) and sell (A) both hold at once.
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
		strategyScriptFound     entities.StrategyScript
		strategyScriptFindError error
		expectedHaltReason      vo.StrategyBotHaltReasonVo
	}{
		{
			name:                    "a strategy script that can no longer be seen",
			strategyScriptFindError: domains.StrategyScriptNotFound(9),
			expectedHaltReason:      vo.StrategyBotHaltStrategyScriptUnavailable,
		},
		{
			// A bot never runs someone else's rules, even ones they still publish.
			name: "a strategy script that belongs to someone else",
			strategyScriptFound: entities.StrategyScript{
				ID: 9, OwnerID: strategyBotOwnerID + 1, Script: "the script", ResultType: "signal",
				Publication: &entities.PublishedStrategyScript{StrategyScriptID: 9},
			},
			expectedHaltReason: vo.StrategyBotHaltStrategyScriptUnavailable,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotRunUnderTest(t)
			underTest.expectDeliverySetting()
			if testCase.strategyScriptFound.Publication != nil {
				underTest.publishedStrategyScriptIDs[testCase.strategyScriptFound.ID] = true
			}

			underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
				Return(testCase.strategyScriptFound, testCase.strategyScriptFindError).AnyTimes()
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
	// No candles yet is a failure that fixes itself, so the bot waits.
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
					// Undelivered means unsaid, so the next round offers the conclusion again.
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
	assert.False(t, roundGuard.TryEnter(strategyBotID))
	assert.True(t, roundGuard.TryEnter(strategyBotID+1))

	roundGuard.Leave(strategyBotID)
	assert.True(t, roundGuard.TryEnter(strategyBotID))
}

func TestStrategyBotRunApplicationSkipsARoundWhoseStoredConditionNoLongerReads(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// A condition naming an undeclared label is not user-correctable, so it skips rather than halts.
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
	// This side failing to ask (not Telegram refusing) waits rather than halting the bot.
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

// One round per bot per scan even when the batch names it twice, independent of round timing.
func TestStrategyBotRunApplicationLeavesABotThatIsAlreadyMidRound(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot(""), aDueBot("")}, nil)
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

			// A failed record is that bot's problem; the scan must not give up on the others.
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

	// Each source stops at the start of its current interval: hourly at 09:00, five-minute at 09:15.
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
	// 這一輪算完之後機器人已被刪除。
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(entities.StrategyBot{}, domains.StrategyBotNotFound(strategyBotID)).AnyTimes()

	underTest.expectNoRoundMessage()

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationDoesNotUndoARestartThatHappenedMidRound(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	// 這一輪開始後擁有者停了又啟動，NextRunAt 被設成現在、上次訊號被清空。
	restarted := aDueBot("")
	restarted.NextRunAt = botRunNow

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(restarted, nil).AnyTimes()

	// 寫回會推遲剛啟動的第一輪並復活被清空的上次訊號，所以一個字都不寫。
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
			// 沒地方講話的機器人要停擺，否則會永遠靜靜地保持「執行中」。
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
			assert.Equal(t, string(vo.StrategyBotHaltDeliveryNotConfigured), bot.HaltReason)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationRunsARoundByHandDownTheSamePath(t *testing.T) {
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
			// 手動跑一輪不會啟動它。
			assert.Equal(t, string(vo.StrategyBotStopped), updated.RunState)

			return nil
		})

	_, runError := underTest.strategyBotRunApplication.RunRoundNow(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, runError)
}

func TestStrategyBotRunApplicationRefusesAHandPressedRoundWhileOneIsInFlight(t *testing.T) {
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

// A bot whose rules are gone halts rather than skips, since waiting fixes nothing.
func TestStrategyBotRunApplicationHaltsABotWhoseRulesAreGone(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{aDueBot("")}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(aDueBot(""), nil)
	*underTest.tradingStrategyFailure = domains.TradingStrategyNotFound(botsTradingStrategyID)
	// The halt message names its reason.
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

// aPositionPlannedDueBot stakes 10% of 50000 with a 3% stop loss and 5% take profit.
func aPositionPlannedDueBot() entities.StrategyBot {
	bot := aDueBot("")
	bot.PositionPlanCapital = decimal.NewFromInt(50000)
	bot.PositionPlanSizingMode = string(vo.PositionSizingModePercentage)
	bot.PositionPlanSizingValue = decimal.NewFromInt(10)
	bot.PositionPlanStopLossPercentage = decimal.NewFromInt(3)
	bot.PositionPlanTakeProfitPercentage = decimal.NewFromInt(5)

	return bot
}

// The planned figures reach both the message and the history, computed once so they cannot disagree.
func TestStrategyBotRunApplicationSuggestsAPositionAndRemembersIt(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.expectDeliverySetting()
	underTest.expectSources(vo.SignalBuy, vo.SignalBuy)

	plannedBot := aPositionPlannedDueBot()
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{plannedBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(plannedBot, nil).AnyTimes()
	underTest.kCandleRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT", 1).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "64180.5")}, nil)

	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "開倉金額 5000")
			assert.Contains(t, message, "止損 62255.085（往下，虧 150）")
			assert.Contains(t, message, "止盈 67389.525（往上，賺 250）")
			assert.Contains(t, message, "這個系統不下單")
			assert.Contains(t, message, "回測要算進止損止盈，重演時把這兩個距離填上")

			return vo.DeliveryFailureNone, nil
		})

	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	underTest.strategyBotRunApplication.RunDueRounds(t.Context())

	require.Len(t, *underTest.appendedRunRecords, 1)
	recorded := (*underTest.appendedRunRecords)[0]
	require.True(t, recorded.HasPositionPlan)
	assert.Equal(t, "5000", recorded.PositionPlan.Stake.String())
	assert.Equal(t, "62255.085", recorded.PositionPlan.StopLossPrice.String())
	assert.Equal(t, "67389.525", recorded.PositionPlan.TakeProfitPrice.String())
}
